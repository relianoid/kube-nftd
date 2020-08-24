package funcs

import (
	"fmt"

	"github.com/zevenet/kube-nftlb/pkg/http"
	"github.com/zevenet/kube-nftlb/pkg/json"
	"github.com/zevenet/kube-nftlb/pkg/logs"
	"github.com/zevenet/kube-nftlb/pkg/types"

	configFarm "github.com/zevenet/kube-nftlb/pkg/farms"
	v1 "k8s.io/api/core/v1"
)

func deleteServiceDsr(farmName string) {
	if _, ok := json.GetDsrArray()[farmName]; ok {
		json.DeleteServiceDsr(farmName)
	}
}

// DeleteNftlbFarm deletes any nftlb farm given a Service object.
func DeleteNftlbFarm(service *v1.Service, logChannel chan string) {
	farmName := ""
	for _, port := range service.Spec.Ports {
		// Get farm name
		if port.Name == "" {
			port.Name = "default"
		}
		farmName = configFarm.AssignFarmNameService(service.ObjectMeta.Name, port.Name)

		// Prints info in the logs about the deleted farm
		response := deleteNftlbRequest(farmName)
		printDeleted("Farm", farmName, "", response, logChannel)
		go json.DeleteMaxConnsMap()

		// check if the farm has mode dsr, if that's the case, clears its stored value in the value map
		go deleteServiceDsr(farmName)

		// Check if the farm is of type nodeport or LB, If that's the case, deleting the original service also deletes the nodePort service.
		// It also deletes its name from the global variable of nodePorts
		if service.Spec.Type == "NodePort" || service.Spec.Type == "LoadBalancer" {
			farmName = configFarm.AssignFarmNameNodePort(farmName, "nodePort")
			nodePortArray := json.GetNodePortArray()
			var indexNodePort = indexOf(farmName, nodePortArray) // delete the name of the service nodeport by index name.
			nodePortArray[indexNodePort] = nodePortArray[len(nodePortArray)-1]
			nodePortArray[len(nodePortArray)-1] = ""
			nodePortArray = nodePortArray[:len(nodePortArray)-1]
			// Prints info in the logs about the deleted farm
			response := deleteNftlbRequest(farmName)
			printDeleted("Farm", farmName, "", response, logChannel)
			// check if the farm type nodeport has mode dsr, if that's the case, clears its stored value in the value map
			go deleteServiceDsr(farmName)
		}
	}
}

// DeleteNftlbBackends deletes all nftlb backends from a farm given a Endpoints object.
func DeleteNftlbBackends(endpoints *v1.Endpoints, logChannel chan string) {
	objName := endpoints.ObjectMeta.Name
	var newServiceNameSlice []string
	for json.GetBackendID(objName) > 0 {
		for _, endpoint := range endpoints.Subsets {
			for _, port := range endpoint.Ports {
				if port.Name != "" {
					newServiceNameSlice = append(newServiceNameSlice, port.Name)
				} else if port.Name == "" {
					newServiceNameSlice = append(newServiceNameSlice, "default")
				}
			}
		}
		for _, serviceName := range newServiceNameSlice {
			// Makes the full path for the request
			backendName := fmt.Sprintf("%s%d", objName, json.GetBackendID(objName))
			fullPath := fmt.Sprintf("%s%s%s/backends/%s", objName, "--", serviceName, backendName)
			response := deleteNftlbRequest(fullPath)
			// Prints info
			printDeleted("Backend", objName, backendName, response, logChannel)
			// Decreases backend ID by 1
			json.DecreaseBackendID(objName)
		}
	}
}

func deleteNftlbRequest(name string) string {
	// Fills the request data
	requestData := &types.RequestData{
		Method: "DELETE",
		Path:   fmt.Sprintf("/%s", name),
	}

	// Get the response from that request
	response, err := http.Send(requestData)
	if err != nil {
		panic(err)
	}

	return string(response)
}

func printDeleted(object string, farmName string, backendName string, response string, logChannel chan string) {
	var message string
	levelLog := 0
	switch object {
	case "Farm":
		message = fmt.Sprintf("\nDeleted %s name: %s\n%s", object, farmName, response)
		logs.PrintLogChannel(levelLog, message, logChannel)
	case "Backend":
		message = fmt.Sprintf("\nDeleted %s:\nFarm: %s, Backend:%s\n%s", object, farmName, backendName, response)
		logs.PrintLogChannel(levelLog, message, logChannel)
	default:
		err := fmt.Sprintf("Unknown deleted object of type %s", object)
		panic(err)
	}
}

func indexOf(element string, data []string) int {
	for k, v := range data {
		if element == v {
			return k
		}
	}
	return -1 //not found.
}
