package messages

import "encoding/xml"

type Tunnels struct {
	XMLName xml.Name `xml:"tunnels"`
	Tunnel  []Tunnel `xml:"tunnel"`
}

type Tunnel struct {
	XMLName     xml.Name `xml:"tunnel"`
	Id          string   `xml:"id,attr"`
	Description string   `xml:"description"`
	ServerV4    string   `xml:"serverv4"`
	ClientV4    string   `xml:"clientv4"`
	ServerV6    string   `xml:"serverv6"`
	ClientV6    string   `xml:"clientv6"`
	Routed64    string   `xml:"routed64"`
	Routed48    string   `xml:"routed48"`
}
