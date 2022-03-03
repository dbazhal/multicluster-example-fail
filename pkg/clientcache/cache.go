package clientcache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	//"k8s.io/client-go/tools/clientcmd"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClientCache struct {
	// local(incluster / k8sconfig) client that main manager works with by default
	localClient client.Client
	// client that is used by local managers/controllers to get some info, typically on startup, typically it's configuration info
	noCacheClient client.Client

	scheme *runtime.Scheme

	// RemoteClients to other clusters. The string is the name of the KubeConfig item targeting
	// another cluster.
	remoteClients map[string]client.Client
}

type FileTokenEntry struct {
	ApiToken string `json:"api_token"`
	ApiUrl   string `json:"api_url"`
}

func New(localClient client.Client, noCacheClient client.Client, scheme *runtime.Scheme) *ClientCache {
	// Call to create new RemoteClients here?
	return &ClientCache{
		localClient:   localClient,
		noCacheClient: noCacheClient,
		scheme:        scheme,
		remoteClients: make(map[string]client.Client),
	}
}

// GetRemoteClient returns the client to remote cluster with name k8sContextName or error if no such client is cached
func (c *ClientCache) GetRemoteClient(apiUrl string) (client.Client, error) {
	if apiUrl == "" {
		return c.localClient, nil
	}

	if cli, found := c.remoteClients[apiUrl]; found {
		fmt.Println("Get client url", apiUrl, "client", cli)
		return cli, nil
	}
	return nil, errors.New("No known client for api " + apiUrl)
}

// GetLocalClient returns the current cluster's client used for operator's local communication
func (c *ClientCache) GetLocalClient() client.Client {
	return c.localClient
}

// GetRemoteClients returns all the remote clients
func (c *ClientCache) GetRemoteClients() map[string]client.Client {
	return c.remoteClients
}

// AddClient adds a new remoteClient with the name k8sContextName
func (c *ClientCache) AddClient(apiUrl string, cli client.Client) {
	fmt.Println("Add client url", apiUrl, "client", cli)
	c.remoteClients[apiUrl] = cli
}

func (c *ClientCache) GetLocalNonCacheClient() client.Client {
	return c.noCacheClient
}

func (c *ClientCache) GetRemoteRestConfigsFromFile(filePath string) (*[]rest.Config, error) {
	var configs []rest.Config
	tokens, err := c.getAdminTokensFromFile(filePath)
	if err != nil {
		return &configs, err
	}
	for _, data := range tokens {
		fmt.Println("API URL", data.ApiUrl)
		singleConfig := rest.Config{
			Host:            data.ApiUrl,
			BearerToken:     data.ApiToken,
			TLSClientConfig: rest.TLSClientConfig{Insecure: true},
		}
		if singleConfig.Host == "" {
			return &configs, errors.New("looks like i failed to load admin tokens - rest config host is empty")
		}
		configs = append(configs, singleConfig)
	}
	return &configs, nil
}

func (c *ClientCache) getAdminTokensFromFile(filePath string) ([]FileTokenEntry, error) {
	var clientsData []FileTokenEntry
	jsonFile, err := os.Open(filePath)
	if err != nil {
		return clientsData, err
	}
	defer func() {
		_ = jsonFile.Close()
	}()
	byteValue, _ := ioutil.ReadAll(jsonFile)

	if err := json.Unmarshal(byteValue, &clientsData); err != nil {
		return clientsData, err
	}
	return clientsData, nil
}
