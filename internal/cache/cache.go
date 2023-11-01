// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cache

import (
	"context"
	"fmt"
	"sync"

	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Cache struct {
	FqdnMap map[string]*types.NamespacedName
	Mu      *sync.RWMutex
}

func NewCache() *Cache {
	return &Cache{
		FqdnMap: make(map[string]*types.NamespacedName),
		Mu:      &sync.RWMutex{},
	}
}

func (c *Cache) PopulateInitialCache(client client.Client) error {
	httpproxyList := &contourv1.HTTPProxyList{}

	if err := client.List(context.Background(), httpproxyList); err != nil {
		return err
	}

	for _, httpproxy := range httpproxyList.Items {
		if httpproxy.Spec.VirtualHost == nil {
			continue
		}

		fqdn := httpproxy.Spec.VirtualHost.Fqdn
		if fqdn != "" {
			_, acquired := c.FqdnMap[fqdn]
			if acquired {
				return fmt.Errorf("FQDN \"%s\" acquired by multiple HTTPProxy objects", fqdn)
			}

			c.FqdnMap[fqdn] = &types.NamespacedName{Namespace: httpproxy.Namespace, Name: httpproxy.Name}
		}
	}

	return nil
}
