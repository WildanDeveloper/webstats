package geo

import (
	"bufio"
	"encoding/binary"
	"net"
	"os"
	"sort"
	"strings"
)

type asnEntry struct {
	lo, hi uint32
	org    string
}

type ASNResolver struct {
	entries []asnEntry
	loaded  bool
}

func LoadASN(path string) (*ASNResolver, error) {
	r := &ASNResolver{}
	if path == "" {
		return r, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return r, nil
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 64*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, ",") {
			continue
		}
		parts := strings.SplitN(line, ",", 2)
		if len(parts) != 2 {
			continue
		}
		netStr := strings.Trim(parts[0], "\"")
		org := strings.Trim(parts[1], "\"")
		if netStr == "" || org == "" {
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
		r.entries = append(r.entries, asnEntry{lo: lo, hi: hi, org: org})
	}
	sort.Slice(r.entries, func(i, j int) bool { return r.entries[i].lo < r.entries[j].lo })
	r.loaded = true
	return r, nil
}

func (r *ASNResolver) Loaded() bool {
	return r != nil && r.loaded
}

func (r *ASNResolver) Org(ipStr string) string {
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
	key := binary.BigEndian.Uint32(v4)
	i := sort.Search(len(r.entries), func(i int) bool { return r.entries[i].hi >= key })
	if i < len(r.entries) && key >= r.entries[i].lo {
		return r.entries[i].org
	}
	return ""
}