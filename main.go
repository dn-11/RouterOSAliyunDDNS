package main

import (
	"awesomeProject/client/provider"
	"awesomeProject/client/provider/aliyun"
	"awesomeProject/client/provider/tencent"
	"awesomeProject/client/ros"
	"awesomeProject/config"
	"log"
	"net/netip"
	"time"
)

var (
	rosClients   = make(map[string]*ros.RouterOSClient)
	cacheAddr    []netip.Addr
	ddnsProvider = make(map[string]provider.DDNSProvider)
)

func main() {
	// 解析配置文件
	if err := config.Parse(); err != nil {
		log.Fatalf("解析配置文件失败: %v", err)
	}

	log.Println("RouterOS DDNS 服务启动")
	log.Printf("更新间隔: %d 秒", config.Conf.Interval)

	// 初始化RouterOS客户端
	for k, c := range config.Conf.RouterOSClient {
		log.Printf("初始化 RouterOS 客户端: %s (%s)", k, c.Host)
		rosClients[k] = ros.NewClient(c)
	}

	// 初始化DDNS提供商
	for k, c := range config.Conf.DDNSProvider {
		providerType := c["type"].(string)
		log.Printf("初始化 DDNS 提供商: %s (%s)", k, providerType)

		switch providerType {
		case "aliyun":
			client, err := aliyun.New(c["access_key_id"].(string), c["access_key_secret"].(string), c["domain"].(string), c["record_key"].(string))
			if err != nil {
				log.Fatalf("创建阿里云 DDNS 客户端失败: %v", err)
			}
			ddnsProvider[k] = client
			log.Printf("阿里云 DDNS 客户端创建成功: %s.%s", c["record_key"].(string), c["domain"].(string))
		case "tencent":
			client, err := tencent.New(c)
			if err != nil {
				log.Fatalf("创建腾讯云 DDNS 客户端失败: %v", err)
			}
			ddnsProvider[k] = client
			log.Printf("腾讯云 DDNS 客户端创建成功: %s.%s", c["subdomain"].(string), c["domain"].(string))
		default:
			log.Fatalf("未知的 DDNS 提供商类型: %s", providerType)
		}
	}
	cacheAddr = make([]netip.Addr, len(config.Conf.Updates))
	log.Printf("配置了 %d 个更新任务", len(config.Conf.Updates))

	// 立即执行一次更新
	update()

	// 设置定时器
	ticker := time.NewTicker(time.Duration(config.Conf.Interval) * time.Second)
	defer ticker.Stop()

	log.Println("进入定时更新循环")
	for range ticker.C {
		update()
	}
}

func update() {
	log.Println("开始执行 DDNS 更新任务")
	updateCount := 0

	for i, v := range config.Conf.Updates {
		log.Printf("执行更新任务 %d: %s -> %s (%s)", i+1, v.From, v.To, v.LoadType)

		rosClient, exists := rosClients[v.From]
		if !exists {
			log.Printf("错误: RouterOS 客户端 '%s' 不存在", v.From)
			continue
		}

		ddnsClient, exists := ddnsProvider[v.To]
		if !exists {
			log.Printf("错误: DDNS 提供商 '%s' 不存在", v.To)
			continue
		}

		var addr netip.Addr
		var err error

		switch v.LoadType {
		case "v4":
			log.Printf("获取 IPv4 地址: %s", v.From)
			addr, err = rosClient.GetIpv4()
			if err != nil {
				log.Printf("获取 IPv4 地址失败 (%s): %v", v.From, err)
				continue
			}
			log.Printf("获取到 IPv4 地址: %s (%s)", addr.String(), v.From)

		case "v6":
			log.Printf("获取 IPv6 地址: %s", v.From)
			addr, err = rosClient.GetIpv6()
			if err != nil {
				log.Printf("获取 IPv6 地址失败 (%s): %v", v.From, err)
				continue
			}
			log.Printf("获取到 IPv6 地址: %s (%s)", addr.String(), v.From)
		default:
			log.Printf("错误: 未知的地址类型: %s", v.LoadType)
			continue
		}

		if cacheAddr[i] == addr {
			log.Printf("地址没有变化，无需更新")
			continue
		}
		cacheAddr[i] = addr

		log.Printf("更新 DDNS 记录: %s -> %s", addr.String(), v.To)
		if err := ddnsClient.Update(addr); err != nil {
			log.Printf("更新 DDNS 记录失败 (%s): %v", v.To, err)
		} else {
			log.Printf("DDNS 记录更新成功: %s (%s)", addr.String(), v.To)
			updateCount++
		}
	}

	log.Printf("本轮更新完成，成功更新 %d/%d 个记录", updateCount, len(config.Conf.Updates))
}
