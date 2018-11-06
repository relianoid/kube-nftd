package watchers

import (
	"fmt"

	funcs "github.com/zevenet/kube-nftlb/pkg/watchers/funcs"
	v1 "k8s.io/api/core/v1"
	fields "k8s.io/apimachinery/pkg/fields"
	runtime "k8s.io/apimachinery/pkg/runtime"
	kubernetes "k8s.io/client-go/kubernetes"
	cache "k8s.io/client-go/tools/cache"
)

// getListWatch makes a ListWatch of every resource in the cluster.
func getListWatch(clientset *kubernetes.Clientset, resource string) *cache.ListWatch {
	listwatch := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(), // REST interface
		resource,                        // Resource to watch for
		v1.NamespaceAll,                 // Resource can be found in ALL namespaces
		fields.Everything(),             // Get ALL fields from requested resource
	)
	return listwatch
}

// getController returns a Controller based on listWatch.
// Exports every message into logChannel.
func getController(listWatch *cache.ListWatch, resourceStruct runtime.Object, resourceName string, logChannel chan string) cache.Controller {
	_, controller := cache.NewInformer(
		listWatch,      // Resources to watch for
		resourceStruct, // Resource struct
		0,
		// Event handler: new, deleted or updated resource
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				funcs.CreateNftlbObject(resourceName, obj)
				logChannel <- fmt.Sprintf("New %s:\n%s\n\n", resourceName, obj)
			},
			DeleteFunc: func(obj interface{}) {
				funcs.DeleteNftlbObject(resourceName, obj)
				logChannel <- fmt.Sprintf("Deleted %s:\n%s\n\n", resourceName, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				funcs.UpdateNftlbObject(resourceName, oldObj, newObj)
				logChannel <- fmt.Sprintf("Updated %s:\nBEFORE:\n%s\nNOW:\n%s\n\n", resourceName, oldObj, newObj)
			},
		},
	)
	return controller
}
