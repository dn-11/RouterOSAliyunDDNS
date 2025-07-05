package tencent

import (
	"errors"
	"fmt"
	errors2 "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"log"
	"net/netip"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/regions"
	client "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"
)

type Tencent struct {
	c         *client.Client
	domain    string
	subdomain string
	hostname  string
}

func New(config map[string]any) (*Tencent, error) {
	accessKeyId := config["access_key_id"].(string)
	accessKeySecret := config["access_key_secret"].(string)
	domain := config["domain"].(string)
	subdomain := config["subdomain"].(string)

	log.Printf("创建腾讯云 DDNS 客户端: %s.%s", subdomain, domain)

	credential := common.NewCredential(accessKeyId, accessKeySecret)

	c, err := client.NewClient(credential, regions.Shanghai, profile.NewClientProfile())
	if err != nil {
		log.Printf("创建腾讯云客户端失败: %v", err)
		return nil, fmt.Errorf("创建腾讯云客户端失败: %v", err)
	}

	tencent := &Tencent{
		c:         c,
		domain:    domain,
		subdomain: subdomain,
		hostname:  fmt.Sprintf("%s.%s", subdomain, domain),
	}

	log.Printf("腾讯云 DDNS 客户端创建成功: %s", tencent.hostname)
	return tencent, nil
}

func (c *Tencent) Update(addr netip.Addr) error {
	log.Printf("开始更新腾讯云 DNS 记录: %s -> %s", c.hostname, addr.String())

	// 确定记录类型
	var recordType string
	if addr.Is6() {
		recordType = "AAAA"
	} else {
		recordType = "A"
	}
	log.Printf("DNS 记录类型: %s", recordType)

	// 查询现有记录
	log.Printf("查询现有 DNS 记录: %s", c.hostname)
	req := client.NewDescribeRecordListRequest()
	req.Domain = &c.domain
	req.Subdomain = &c.subdomain
	resp, err := c.c.DescribeRecordList(req)
	if err != nil {
		var errTencent *errors2.TencentCloudSDKError
		if !errors.As(err, &errTencent) || errTencent.GetCode() != "ResourceNotFound.NoDataOfRecord" {
			log.Printf("查询 DNS 记录失败: %v", err)
			return fmt.Errorf("查询记录失败: %v", err)
		}
	}

	var n int
	if resp.Response != nil {
		n = len(resp.Response.RecordList)
	} else {
		n = 0
	}

	log.Printf("查询到 %d 条记录", n)

	var record *client.RecordListItem
	if n != 0 {
		for _, r := range resp.Response.RecordList {
			if r.Type != nil && *r.Type == recordType {
				record = r
				log.Printf("找到匹配的 %s 记录: %s", recordType, *r.Value)
				break
			}
		}
	}

	if record == nil {
		// 创建新记录
		log.Printf("创建新的 DNS 记录: %s = %s (%s)", c.hostname, addr.String(), recordType)
		req := client.NewCreateRecordRequest()
		req.Domain = &c.domain
		req.RecordType = &recordType
		req.RecordLine = common.StringPtr("默认")
		req.Value = common.StringPtr(addr.String())
		req.SubDomain = &c.subdomain
		_, err := c.c.CreateRecord(req)
		if err != nil {
			log.Printf("创建 DNS 记录失败: %v", err)
			return fmt.Errorf("创建记录失败: %v", err)
		}
		log.Printf("DNS 记录创建成功: %s = %s", c.hostname, addr.String())
		return nil
	}

	// 检查是否需要更新
	currentValue := *record.Value
	log.Printf("当前记录值: %s", currentValue)

	if currentValue == addr.String() {
		log.Printf("DNS 记录已是最新，无需更新: %s", addr.String())
		return nil
	}

	// 更新现有记录
	log.Printf("更新现有 DNS 记录: %s -> %s", currentValue, addr.String())
	reqModDNS := client.NewModifyRecordRequest()
	reqModDNS.Domain = common.StringPtr(c.domain)
	reqModDNS.RecordId = record.RecordId
	reqModDNS.RecordLine = record.Line
	reqModDNS.Value = common.StringPtr(addr.String())
	reqModDNS.RecordType = record.Type
	reqModDNS.SubDomain = record.Name
	_, err = c.c.ModifyRecord(reqModDNS)
	if err != nil {
		log.Printf("更新 DNS 记录失败: %v", err)
		return fmt.Errorf("更新记录失败: %v", err)
	}

	log.Printf("DNS 记录更新成功: %s = %s", c.hostname, addr.String())
	return nil
}
