package aliyun

import (
	"fmt"
	"github.com/alibabacloud-go/tea/tea"
	"log"
	"net/netip"

	alidns20150109 "github.com/alibabacloud-go/alidns-20150109/v4/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
)

type AliyunDDNSClient struct {
	client *alidns20150109.Client

	domain    string
	recordKey string
	hostname  string
}

func New(secretID, secretKey, domain, recordKey string) (*AliyunDDNSClient, error) {
	log.Printf("创建阿里云 DDNS 客户端: %s.%s", recordKey, domain)

	aliyun, err := alidns20150109.NewClient(&openapi.Config{
		AccessKeyId:     &secretID,
		AccessKeySecret: &secretKey,
	})
	if err != nil {
		log.Printf("创建阿里云客户端失败: %v", err)
		return nil, fmt.Errorf("spawn aliyun ddns client: %v", err)
	}

	client := &AliyunDDNSClient{
		client:    aliyun,
		domain:    domain,
		recordKey: recordKey,
		hostname:  fmt.Sprintf("%s.%s", recordKey, domain),
	}

	log.Printf("阿里云 DDNS 客户端创建成功: %s", client.hostname)
	return client, nil
}

func (c *AliyunDDNSClient) Update(ip netip.Addr) error {
	log.Printf("开始更新阿里云 DNS 记录: %s -> %s", c.hostname, ip.String())

	// 确定记录类型
	recordType := "A"
	if ip.Is6() {
		recordType = "AAAA"
	}
	log.Printf("DNS 记录类型: %s", recordType)

	// 查询现有记录
	log.Printf("查询现有 DNS 记录: %s", c.hostname)
	describeReq := &alidns20150109.DescribeDomainRecordsRequest{
		DomainName: tea.String(c.domain),
		RRKeyWord:  tea.String(c.recordKey),
		Type:       tea.String(recordType),
	}

	resp, err := c.client.DescribeDomainRecords(describeReq)
	if err != nil {
		log.Printf("查询 DNS 记录失败: %v", err)
		return fmt.Errorf("查询记录失败: %v", err)
	}

	records := resp.Body.DomainRecords.Record
	log.Printf("找到 %d 条现有记录", len(records))

	// 检查是否需要更新
	if len(records) > 0 {
		existingRecord := records[0]
		currentValue := *existingRecord.Value
		log.Printf("当前记录值: %s", currentValue)

		if currentValue == ip.String() {
			log.Printf("DNS 记录已是最新，无需更新: %s", ip.String())
			return nil
		}

		// 更新现有记录
		log.Printf("更新现有 DNS 记录: %s -> %s", currentValue, ip.String())
		updateReq := &alidns20150109.UpdateDomainRecordRequest{
			RecordId: existingRecord.RecordId,
			RR:       tea.String(c.recordKey),
			Type:     tea.String(recordType),
			Value:    tea.String(ip.String()),
		}

		_, err = c.client.UpdateDomainRecord(updateReq)
		if err != nil {
			log.Printf("更新 DNS 记录失败: %v", err)
			return fmt.Errorf("更新记录失败: %v", err)
		}

		log.Printf("DNS 记录更新成功: %s = %s", c.hostname, ip.String())
	} else {
		// 创建新记录
		log.Printf("创建新的 DNS 记录: %s = %s", c.hostname, ip.String())
		addReq := &alidns20150109.AddDomainRecordRequest{
			DomainName: tea.String(c.domain),
			RR:         tea.String(c.recordKey),
			Type:       tea.String(recordType),
			Value:      tea.String(ip.String()),
		}

		_, err = c.client.AddDomainRecord(addReq)
		if err != nil {
			log.Printf("创建 DNS 记录失败: %v", err)
			return fmt.Errorf("创建记录失败: %v", err)
		}

		log.Printf("DNS 记录创建成功: %s = %s", c.hostname, ip.String())
	}

	return nil
}
