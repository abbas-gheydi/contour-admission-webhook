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
	"sync"
	"time"

	"github.com/snapp-incubator/contour-admission-webhook/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	logger = ctrl.Log.WithName("cache")
)

type Cache struct {
	fqdnMap         map[string]*element // map[ingressClassName/FQDN]*element
	mu              *sync.RWMutex
	cleanUpTicker   *time.Ticker // Ticker
	CleanUpStopChan chan bool    // Channel for stopping the ticker
}

type element struct {
	Value     *types.NamespacedName
	ExpiresAt int64
}

func NewCache(cleanUpInterval time.Duration) *Cache {
	cache := &Cache{
		fqdnMap:         make(map[string]*element),
		mu:              &sync.RWMutex{},
		cleanUpTicker:   time.NewTicker(cleanUpInterval),
		CleanUpStopChan: make(chan bool),
	}

	cache.StartCleaner()

	return cache
}

func (c *Cache) Set(key string, value *types.NamespacedName, expirationUnixTime int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.fqdnMap[key] = &element{
		Value:     value,
		ExpiresAt: expirationUnixTime,
	}
}

func (c *Cache) Get(key string) (*types.NamespacedName, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	element, found := c.fqdnMap[key]
	if !found {
		return nil, false
	}

	return element.Value, true
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.fqdnMap, key)
}

func (c *Cache) KeyExists(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, found := c.fqdnMap[key]

	return found
}

func (c *Cache) IsKeyPersisted(key string) *bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, found := c.fqdnMap[key]
	if !found {
		return nil
	}

	return utils.BoolPointer(entry.ExpiresAt == 0)
}

func (c *Cache) StartCleaner() {
	go func() {
	out:
		for {
			select {
			case <-c.cleanUpTicker.C:
				c.cleanUp()
			case <-c.CleanUpStopChan:
				break out
			}
		}
	}()
}

func (c *Cache) cleanUp() {
	now := time.Now().Unix()

	c.mu.Lock()
	defer c.mu.Unlock()

	for key, element := range c.fqdnMap {
		if element.ExpiresAt > 0 && now >= element.ExpiresAt {
			delete(c.fqdnMap, key)

			logger.Info("cache entry is expired hence deleted", "entry", key)
		}
	}
}
