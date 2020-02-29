package local

import (
	"net"

	"github.com/jackpal/gateway"
	natpmp "github.com/jackpal/go-nat-pmp"
)

func configureNATPMP(autopeeringPort int, networkingPort int) (externalIP net.IP) {

	//TODO: this probably needs a timer every 3600 seconds to renew the port mappings

	gatewayIP, err := gateway.DiscoverGateway()
	if err != nil {
		log.Errorf("Error getting gateway: %v", err)
	} else {
		client := natpmp.NewClient(gatewayIP)

		response, err := client.GetExternalAddress()
		if err != nil {
			log.Fatalf("Error querying external IP address: %v", err)
		}

		externalIP = net.IPv4(response.ExternalIPAddress[0], response.ExternalIPAddress[1], response.ExternalIPAddress[2], response.ExternalIPAddress[3])
		log.Infof("External IP address: %v", externalIP.String())

		result, err := client.AddPortMapping("udp", autopeeringPort, autopeeringPort, 3600)
		if err != nil {
			log.Fatalf("Error adding autopeering port mapping: %v", err)
		}

		log.Infof("Added autopeering port mapping to %s:%v", externalIP.String(), result.MappedExternalPort)

		result, err = client.AddPortMapping("tcp", networkingPort, networkingPort, 3600)
		if err != nil {
			log.Fatalf("Error adding networking port mapping: %v", err)
		}

		log.Infof("Added networking port mapping to %s:%v", externalIP.String(), result.MappedExternalPort)
	}
	return
}

func cleanupNATPMP(autopeeringPort int, networkingPort int) {
	gatewayIP, err := gateway.DiscoverGateway()
	if err != nil {
		log.Errorf("Error getting gateway: %v", err)
	} else {
		client := natpmp.NewClient(gatewayIP)

		_, err := client.AddPortMapping("udp", autopeeringPort, 0, 0)
		if err != nil {
			log.Fatalf("Error adding autopeering port mapping: %v", err)
		}

		log.Info("Removed autopeering port mapping")

		_, err = client.AddPortMapping("tcp", networkingPort, 0, 0)
		if err != nil {
			log.Fatalf("Error adding networking port mapping: %v", err)
		}

		log.Info("Removed networking port mapping")
	}
}
