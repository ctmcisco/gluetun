package updater

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/qdm12/gluetun/internal/models"
	"github.com/qdm12/golibs/network"
)

func (u *updater) updatePurevpn(ctx context.Context) (err error) {
	servers, warnings, err := findPurevpnServers(ctx, u.client, u.lookupIP)
	if u.options.CLI {
		for _, warning := range warnings {
			u.logger.Warn("PureVPN: %s", warning)
		}
	}
	if err != nil {
		return fmt.Errorf("cannot update Purevpn servers: %w", err)
	}
	if u.options.Stdout {
		u.println(stringifyPurevpnServers(servers))
	}
	u.servers.Purevpn.Timestamp = u.timeNow().Unix()
	u.servers.Purevpn.Servers = servers
	return nil
}

func findPurevpnServers(ctx context.Context, client network.Client, lookupIP lookupIPFunc) (
	servers []models.PurevpnServer, warnings []string, err error) {
	const zipURL = "https://s3-us-west-1.amazonaws.com/heartbleed/windows/New+OVPN+Files.zip"
	contents, err := fetchAndExtractFiles(ctx, client, zipURL)
	if err != nil {
		return nil, nil, err
	}

	hosts := make([]string, 0, len(contents))
	for fileName, content := range contents {
		if strings.HasSuffix(fileName, "-tcp.ovpn") {
			continue // only parse UDP files
		}
		host, warning, err := extractHostFromOVPN(content)
		if len(warning) > 0 {
			warnings = append(warnings, warning)
		}
		if err != nil {
			return nil, warnings, fmt.Errorf("%w in %q", err, fileName)
		}
		hosts = append(hosts, host)
	}

	const repetition = 20
	const timeBetween = time.Second
	const failOnErr = true
	hostToIPs, _, err := parallelResolve(ctx, lookupIP, hosts, repetition, timeBetween, failOnErr)
	if err != nil {
		return nil, warnings, err
	}

	uniqueServers := make(map[string]models.PurevpnServer, len(hostToIPs))
	for host, IPs := range hostToIPs {
		if len(IPs) == 0 {
			warning := fmt.Sprintf("no IP address found for host %q", host)
			warnings = append(warnings, warning)
			continue
		}
		country, region, city, err := getIPInfo(ctx, client, IPs[0])
		if err != nil {
			return nil, warnings, err
		}
		key := country + region + city
		server, ok := uniqueServers[key]
		if ok {
			server.IPs = append(server.IPs, IPs...)
		} else {
			server = models.PurevpnServer{
				Country: country,
				Region:  region,
				City:    city,
				IPs:     IPs,
			}
		}
		uniqueServers[key] = server
	}

	servers = make([]models.PurevpnServer, len(uniqueServers))
	i := 0
	for _, server := range uniqueServers {
		servers[i] = server
		i++
	}

	sort.Slice(servers, func(i, j int) bool {
		if servers[i].Country == servers[j].Country {
			if servers[i].Region == servers[j].Region {
				return servers[i].City < servers[j].City
			}
			return servers[i].Region < servers[j].Region
		}
		return servers[i].Country < servers[j].Country
	})

	return servers, warnings, nil
}

func stringifyPurevpnServers(servers []models.PurevpnServer) (s string) {
	s = "func PurevpnServers() []models.PurevpnServer {\n"
	s += "	return []models.PurevpnServer{\n"
	for _, server := range servers {
		s += "		" + server.String() + ",\n"
	}
	s += "	}\n"
	s += "}"
	return s
}
