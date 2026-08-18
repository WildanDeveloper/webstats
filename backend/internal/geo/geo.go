package geo

import (
	"bufio"
	"encoding/binary"
	"net"
	"os"
	"sort"
	"strings"
)

type entry struct {
	lo, hi uint32
	cc     string
}

type Resolver struct {
	entries []entry
	loaded  bool
}

// Load parses a GeoLite2-Country-Locations CSV ("network,geoname_id,...").
// It is optional: without a file, lookups return "".
func Load(path string) (*Resolver, error) {
	r := &Resolver{}
	if path == "" {
		return r, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return r, nil // silent fallback, file is optional
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 64*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, ",") {
			continue
		}
		netStr, cc := parseLine(line)
		if netStr == "" || cc == "" {
			continue
		}
		_, ipNet, err := net.ParseCIDR(netStr)
		if err != nil {
			continue
		}
		lo, hi := rangeToUint32(ipNet)
		if lo == 0 {
			continue
		}
		r.entries = append(r.entries, entry{lo: lo, hi: hi, cc: cc})
	}
	sort.Slice(r.entries, func(i, j int) bool { return r.entries[i].lo < r.entries[j].lo })
	r.loaded = true
	return r, nil
}

func parseLine(line string) (netStr, cc string) {
	parts := strings.SplitN(line, ",", 2)
	if len(parts) != 2 {
		return "", ""
	}
	netStr = strings.Trim(parts[0], "\"")
	cc = strings.Trim(parts[1], "\"")
	return netStr, cc
}

func rangeToUint32(n *net.IPNet) (uint32, uint32) {
	ip := n.IP.To4()
	if ip == nil {
		return 0, 0
	}
	mask := binary.BigEndian.Uint32(n.Mask)
	base := binary.BigEndian.Uint32(ip) & mask
	ones, bits := n.Mask.Size()
	host := uint32(0)
	if bits == 32 {
		host = 1<<uint(32-ones) - 1
	}
	hi := base | host
	if hi == 0 {
		hi = base
	}
	return base, hi
}

func (r *Resolver) CountryCode(ipStr string) string {
	if r == nil || len(r.entries) == 0 {
		return ""
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}
	v4 := ip.To4()
	if v4 == nil {
		return ""
	}
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return "local"
	}
	n := binary.BigEndian.Uint32(v4)
	i := sort.Search(len(r.entries), func(i int) bool { return r.entries[i].hi >= n })
	if i < len(r.entries) && r.entries[i].lo <= n {
		return r.entries[i].cc
	}
	return ""
}

func (r *Resolver) Loaded() bool { return r.loaded }
