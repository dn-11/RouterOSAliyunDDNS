package ros

import (
	"awesomeProject/config"
	"errors"
	"fmt"
	"log"
	"net/netip"
	"slices"
	"time"

	"github.com/go-routeros/routeros/v3/proto"

	"github.com/go-routeros/routeros/v3"
)

type RouterOSClient struct {
	host  string
	user  string
	pass  string
	iface string
}

func NewClient(conf config.RouterOSClientConfig) *RouterOSClient {
	client := &RouterOSClient{
		host:  conf.Host,
		user:  conf.User,
		pass:  conf.Password,
		iface: conf.Interface,
	}
	log.Printf("创建 RouterOS 客户端: %s@%s (接口: %s)", conf.User, conf.Host, conf.Interface)
	return client
}

func (r *RouterOSClient) GetIpv4() (netip.Addr, error) {
	log.Printf("连接到 RouterOS: %s", r.host)
	c, err := routeros.DialTimeout(r.host, r.user, r.pass, time.Second*20)
	if err != nil {
		log.Printf("连接 RouterOS 失败 (%s): %v", r.host, err)
		return netip.Addr{}, fmt.Errorf("连接失败: %v", err)
	}
	defer c.Close()
	log.Printf("RouterOS 连接成功，查询接口 %s 的 IPv4 地址", r.iface)

	reply, err := c.Run("/ip/address/print", "?=interface="+r.iface)
	if err != nil {
		log.Printf("查询 IPv4 地址失败 (%s): %v", r.iface, err)
		return netip.Addr{}, fmt.Errorf("查询地址失败: %v", err)
	}

	log.Printf("收到 %d 个 IPv4 地址响应", len(reply.Re))
	addr, err := filterAddr(reply.Re)
	if err != nil {
		log.Printf("过滤 IPv4 地址失败: %v", err)
		return netip.Addr{}, err
	}

	log.Printf("成功获取 IPv4 地址: %s", addr.String())
	return addr, nil
}

func (r *RouterOSClient) GetIpv6() (netip.Addr, error) {
	log.Printf("连接到 RouterOS: %s", r.host)
	c, err := routeros.DialTimeout(r.host, r.user, r.pass, time.Second*20)
	if err != nil {
		log.Printf("连接 RouterOS 失败 (%s): %v", r.host, err)
		return netip.Addr{}, fmt.Errorf("连接失败: %v", err)
	}
	defer c.Close()
	log.Printf("RouterOS 连接成功，查询接口 %s 的 IPv6 地址", r.iface)

	reply, err := c.Run("/ipv6/address/print", "?=interface="+r.iface)
	if err != nil {
		log.Printf("查询 IPv6 地址失败 (%s): %v", r.iface, err)
		return netip.Addr{}, fmt.Errorf("查询地址失败: %v", err)
	}

	log.Printf("收到 %d 个 IPv6 地址响应", len(reply.Re))
	addr, err := filterAddr(reply.Re)
	if err != nil {
		log.Printf("过滤 IPv6 地址失败: %v", err)
		return netip.Addr{}, err
	}

	log.Printf("成功获取 IPv6 地址: %s", addr.String())
	return addr, nil
}

func filterAddr(replies []*proto.Sentence) (netip.Addr, error) {
	log.Printf("开始过滤地址，原始数量: %d", len(replies))

	// 记录过滤前的地址
	for i, sentence := range replies {
		if addr := sentence.Map["address"]; addr != "" {
			log.Printf("地址 %d: %s", i+1, addr)
		}
	}

	replies = slices.DeleteFunc(replies, func(sentence *proto.Sentence) bool {
		addr, err := netip.ParsePrefix(sentence.Map["address"])
		if err != nil {
			log.Printf("解析地址失败，跳过: %s", sentence.Map["address"])
			return true
		}
		if addr.Addr().IsPrivate() {
			log.Printf("私有地址，跳过: %s", addr.Addr().String())
			return true
		}
		if addr.Addr().IsLinkLocalUnicast() {
			log.Printf("链路本地地址，跳过: %s", addr.Addr().String())
			return true
		}
		return false
	})

	log.Printf("过滤后地址数量: %d", len(replies))

	if len(replies) == 0 {
		return netip.Addr{}, errors.New("没有找到可用的公网地址")
	}

	if len(replies) != 1 {
		log.Printf("警告: 找到多个公网地址 (%d 个)，使用第一个", len(replies))
		for i, sentence := range replies {
			log.Printf("候选地址 %d: %s", i+1, sentence.Map["address"])
		}
	}

	address := replies[0].Map["address"]
	prefix, err := netip.ParsePrefix(address)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("解析最终地址失败: %v", err)
	}

	return prefix.Addr(), nil
}
