package funcs

import (
	"fmt"
	"strings"

	"github.com/zevenet/kube-nftlb/pkg/http"
	"github.com/zevenet/kube-nftlb/pkg/json"
	"github.com/zevenet/kube-nftlb/pkg/logs"
	"github.com/zevenet/kube-nftlb/pkg/types"
	"k8s.io/client-go/kubernetes"

	configFarm "github.com/zevenet/kube-nftlb/pkg/farms"
	v1 "k8s.io/api/core/v1"
)

// UpdateNftlbFarm updates any nftlb farm given a Service object.
func UpdateNftlbFarm(newSvc *v1.Service, clientset *kubernetes.Clientset) {
	if !json.Contains(http.BadNames, newSvc.ObjectMeta.Name) {
		// Translates the updated Service object into a JSONnftlb struct
		newJSONnftlb := json.GetJSONnftlbFromService(newSvc, clientset)
		// Translates that struct into a JSON string
		farmJSON := json.DecodePrettyJSON(newJSONnftlb)
		// Makes the request
		response := updateNftlbRequest(farmJSON)
		// Prints info
		printUpdated("Farm", farmJSON, response)
	}
}

// UpdateNftlbBackends updates backends for any farm given a Endpoints object.
func UpdateNftlbBackends(oldEP, newEP *v1.Endpoints, clientset *kubernetes.Clientset) {
	if !json.Contains(http.BadNames, newEP.ObjectMeta.Name) {
		// Gets the service and number of backends for later
		objName := oldEP.ObjectMeta.Name
		// Translates the Endpoints objects into JSONnftlb structs
		newJSONnftlb := json.GetJSONnftlbFromEndpoints(newEP, clientset)
		// Translates the struct into a JSON string
		backendsJSON := json.DecodePrettyJSON(newJSONnftlb)
		// We create an array with the old object (what was there before updating) and with the current object (after updating)
		// We will use it to compare which backends have been removed and, based on them, update the backends of the corresponding services
		// In addition we will also store the name of the service to know what service exactly needs to be updated
		var newBackendsNameSlice = getNewBackendsSlice(newEP)
		var newServiceNameSlice = getNewServiceSlice(newEP)
		var oldServiceNameSlice = getOldServiceSlice(oldEP)
		// Makes the request nftlb
		response := updateNftlbRequest(backendsJSON)
		printUpdated("Backends", backendsJSON, response)
		farmName := ""
		backendName := ""
		// There are two possible situations.
		// The first situation is where there is only one defined backend and that backend is removed.
		// As both arrays cannot be compared in this case, all backends associated with it in its old object are removed (oldServiceNameSlice)
		for _, endpoint := range oldEP.Subsets {
			for _, address := range endpoint.Addresses {
				if address.TargetRef != nil {
					if len(newServiceNameSlice) < 1 {
						for _, serviceName := range oldServiceNameSlice {
							farmName = configFarm.AssignFarmNameService(objName, serviceName)
							backendName = fmt.Sprintf("%s", address.TargetRef.Name)
							actionDeleteNftlbRequest(objName, farmName, backendName)
						}
						// 	The second situation is where multiple backends are defined and some or all of them are removed.
						//  Both arrays are compared and we check that the backend has been removed. Once detected, all necessary backends are removed.
					} else {
						// Find Missing backends in the slice of backends.
						// If the backend is not in the slice of new backends it is removed
						_, found := find(newBackendsNameSlice, address.TargetRef.Name)
						if !found {
							for _, serviceName := range newServiceNameSlice {
								farmName = configFarm.AssignFarmNameService(objName, serviceName)
								backendName = fmt.Sprintf("%s", address.TargetRef.Name)
								actionDeleteNftlbRequest(objName, farmName, backendName)
							}
						}
					}
				}
			}
		}
	}
}

func actionDeleteNftlbRequest(objName string, farmName string, backendName string) {
	// We create the full path to remove the backend. To do this we have to indicate which farm contains the backend
	fullPath := fmt.Sprintf("%s/backends/%s", farmName, backendName)
	response := deleteNftlbRequest(fullPath)
	printDeleted("Backend", objName, backendName, response)
	// Check if the current service is of type nodeport
	// If this is the case, delete the backends also next to those of the service
	farmNodeport := configFarm.AssignFarmNameNodePort(farmName, "nodePort")
	if json.Contains(json.GetNodePortArray(), farmNodeport) {
		fullPath = fmt.Sprintf("%s/backends/%s", farmNodeport, backendName)
		response = deleteNftlbRequest(fullPath)
		printDeleted("Backend", objName, backendName, response)
	}
	// Check if the current service is of type externalIPs
	// If this is the case, delete the backends also next to those of the service
	if _, ok := json.GetExternalIPsArray()[farmName]; ok {
		for _, farmExternalIPs := range json.GetExternalIPsArray()[farmName] {
			fullPath = fmt.Sprintf("%s/backends/%s", farmExternalIPs, backendName)
			response = deleteNftlbRequest(fullPath)
			printDeleted("Backend", objName, backendName, response)
		}
	}
}

func updateNftlbRequest(json string) string {
	// Fills the request
	requestData := &types.RequestData{
		Method: "POST",
		Body:   strings.NewReader(json),
	}

	// Get the response from that request
	response, err := http.Send(requestData)
	if err != nil {
		panic(err)
	}

	return string(response)
}

func printUpdated(object string, json string, response string) {
	levelLog := 0
	message := fmt.Sprintf("\nUpdated %s:\n%s\n%s", object, json, response)
	logs.WriteLog(levelLog, message)
}

func find(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

func getNewBackendsSlice(newEP *v1.Endpoints) []string {
	// Loops through the current endpoint object and stores the names of the backends that are currently created.
	// In other words, after deleting or increasing the backends, this object contains all the information related to the backends.
	var newBackendsNameSlice []string
	for _, endpoint := range newEP.Subsets {
		for _, address := range endpoint.Addresses {
			if address.TargetRef != nil {
				newBackendsNameSlice = append(newBackendsNameSlice, address.TargetRef.Name)
			}
		}
	}
	return newBackendsNameSlice
}

func getNewServiceSlice(newEP *v1.Endpoints) []string {
	// Loops through the current endpoint object and stores the service name
	// In other words, we store the name of the service and then reference it and then get the name of our farm (which will help us in the deletion process, where we need to refer to the name of the farm to delete the backend)
	var newServiceNameSlice []string
	for _, endpoint := range newEP.Subsets {
		for _, port := range endpoint.Ports {
			if port.Name != "" {
				newServiceNameSlice = append(newServiceNameSlice, port.Name)
			} else if port.Name == "" {
				newServiceNameSlice = append(newServiceNameSlice, "default")
			}
		}
	}
	return newServiceNameSlice
}

func getOldServiceSlice(oldEP *v1.Endpoints) []string {
	// Loops through the old endpoint object and stores the service name
	// In this case we store the names of the old services and then make reference and delete our backends (see first situation)
	var oldServiceNameSlice []string
	for _, endpoint := range oldEP.Subsets {
		for _, port := range endpoint.Ports {
			if port.Name != "" {
				oldServiceNameSlice = append(oldServiceNameSlice, port.Name)
			} else if port.Name == "" {
				oldServiceNameSlice = append(oldServiceNameSlice, "default")
			}
		}
	}
	return oldServiceNameSlice
}
