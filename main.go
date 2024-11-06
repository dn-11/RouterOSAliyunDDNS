package main

import (
	"errors"
	"flag"
	"fmt"
	alidns20150109 "github.com/alibabacloud-go/alidns-20150109/v4/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/go-routeros/routeros/v3"
	"log"
	"net"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"time"
)

var (
	routerOSAddr      string
	routerOSUser      string
	routerOSPwd       string
	routerOSInterface string
	accessKeyId       string
	accessKeySecret   string
	recordType        string
	domain            string
	recordKey         string
	seconds           int
)

func main() {
	flag.StringVar(&routerOSAddr, "rosa", os.Getenv("ROS_ADDR"), "RouterOS API Address")
	flag.StringVar(&routerOSUser, "rosu", os.Getenv("ROS_USERNAME"), "RouterOS API Username")
	flag.StringVar(&routerOSPwd, "rosp", os.Getenv("ROS_PASSWORD"), "RouterOS API Password")
	flag.StringVar(&routerOSInterface, "rosi", func() string {
		if os.Getenv("ROS_INTERFACE") != "" {
			return os.Getenv("ROS_INTERFACE")
		}
		return "WAN"
	}(), "RouterOS Interface")
	flag.IntVar(&seconds, "i", func() int {
		if os.Getenv("INTERVAL") != "" {
			i, err := strconv.Atoi(os.Getenv("INTERVAL"))
			if err == nil {
				return i
			}
		}
		return 60
	}(), "interval in seconds")
	flag.StringVar(&accessKeyId, "ak", os.Getenv("ALIYUN_AK"), "Aliyun Access Key ID")
	flag.StringVar(&accessKeySecret, "sk", os.Getenv("ALIYUN_SK"), "Aliyun Access Key Secret")
	flag.StringVar(&recordType, "t", func() string {
		if os.Getenv("RECORD_TYPE") != "" {
			return os.Getenv("RECORD_TYPE")
		}
		return "A"
	}(), "Record Type")
	flag.StringVar(&domain, "d", os.Getenv("DOMAIN"), "Domain")
	flag.StringVar(&recordKey, "k", os.Getenv("RECORD_KEY"), "Record Key")
	flag.Parse()

	Required("RouterOS Address", routerOSAddr)
	Required("RouterOS Username", routerOSUser)
	Required("RouterOS Password", routerOSPwd)
	Required("RouterOS Interface", routerOSInterface)
	Required("Aliyun Access Key ID", accessKeyId)
	Required("Aliyun Access Key Secret", accessKeySecret)
	Required("Domain", domain)
	Required("Record Key", recordKey)

	ticker := time.NewTicker(time.Duration(seconds) * time.Second)
	update()
	for range ticker.C {
		update()
	}
}

func Required(key, value string) {
	if value == "" {
		log.Fatalf("%s is required", key)
	}
}

func update() {
	hostname := fmt.Sprintf("%s.%s", recordKey, domain)
	aliyun, err := alidns20150109.NewClient(&openapi.Config{
		AccessKeyId:     &accessKeyId,
		AccessKeySecret: &accessKeySecret,
	})
	if err != nil {
		log.Fatalln(err)
	}
	ip, err := GetIpFromMitroTik()
	if err != nil {
		log.Println("getIpFromMitroTik failed:", err)
		return
	}
	log.Printf("get current ip %s", ip.String())

	lookupIP, err := net.LookupIP(hostname)
	if err != nil {
		log.Println("lookupIP failed:", err)
		return
	}

	if len(lookupIP) > 1 {
		log.Println("too many resolves to the domain name")
	}

	if len(lookupIP) == 1 {
		dnsip := lookupIP[0]
		if netip.MustParseAddr(dnsip.String()) == *ip {
			log.Println("no need to update")
			return
		}
	}

	res, err := aliyun.DescribeDomainRecords(&alidns20150109.DescribeDomainRecordsRequest{
		DomainName:  &domain,
		RRKeyWord:   &recordKey,
		TypeKeyWord: &recordType,
	})
	if err != nil {
		log.Println("describeDomainRecords failed:", err)
		return
	}
	records := res.Body.DomainRecords.Record
	if len(records) < 1 {
		log.Println("no records found")
		res, err := aliyun.AddDomainRecord(&alidns20150109.AddDomainRecordRequest{
			DomainName: &domain,
			RR:         &recordKey,
			Type:       &recordType,
			Value:      tea.String(ip.String()),
		})
		if err != nil {
			log.Println("addDomainRecord failed:", err)
			return
		}
		log.Printf("add Record %s %s(%s)", *res.Body.RecordId, hostname, ip.String())
		return
	}

	idx := slices.IndexFunc(records, func(record *alidns20150109.DescribeDomainRecordsResponseBodyDomainRecordsRecord) bool {
		return *record.Value == ip.String()
	})
	if idx == -1 {
		record := records[0]
		_, err = aliyun.UpdateDomainRecord(&alidns20150109.UpdateDomainRecordRequest{
			RecordId: record.RecordId,
			RR:       &recordKey,
			Type:     &recordType,
			Value:    tea.String(ip.String()),
		})
		if err != nil {
			log.Println("updateDomainRecord failed:", err.Error())
			return
		}
		log.Printf("update Record %s %s(%s)", *record.RecordId, hostname, ip.String())
		records = records[1:]
	} else {
		records = append(records[:idx], records[idx+1:]...)
	}

	for _, v := range records {
		_, err := aliyun.DeleteDomainRecord(&alidns20150109.DeleteDomainRecordRequest{
			RecordId: v.RecordId,
		})
		if err != nil {
			log.Printf("delete record %s failed", *v.RecordId)
		}
	}
}

func GetIpFromMitroTik() (*netip.Addr, error) {
	c, err := routeros.DialTimeout(routerOSAddr, routerOSUser, routerOSPwd, time.Second*20)
	if err != nil {
		return nil, err
	}
	reply, err := c.Run("/ip/address/print", "?=interface="+routerOSInterface)
	if err != nil {
		return nil, err
	}
	if len(reply.Re) != 1 {
		return nil, errors.New("WAN interface not unique")
	}
	address := reply.Re[0].Map["address"]
	prefix, err := netip.ParsePrefix(address)
	if err != nil {
		return nil, err
	}
	addr := prefix.Addr()
	return &addr, nil
}
