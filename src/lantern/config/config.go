/*
Package config encapsulates the configuration for this lantern node, which is
backed by a config.json file stored on the file system.

The config.json is found in [ConfigDir].

[ConfigDir] defaults to ~/.lantern, so by default the config.json file is
expected to be located at ~/.lantern/config.json.

A different [ConfigDir] can be used by specifying it as the first argument to
the lantern command.
*/
package config

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os/user"
	"sync"
)

/*
ParentAddress() returns the host:port at which this lantern instance should
try to connect to its parent node.

A blank value means that this lantern instance is a root node
*/
func ParentAddress() string {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return config.ParentAddress
}

// IsRootNode() indicates whether or not this lantern node is a root
func IsRootNode() bool {
	return ParentAddress() == ""
}

func SetParentAddress(parentAddress string) {
	configMutex.Lock()
	defer configMutex.Unlock()
	config.ParentAddress = parentAddress
	save()
}

// SignalingAddress() returns the host:port at which this lantern node is
// listening for signaling channel connections.
func SignalingAddress() string {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return config.SignalingAddress
}

func SetSignalingAddress(signalingAddress string) {
	configMutex.Lock()
	defer configMutex.Unlock()
	config.SignalingAddress = signalingAddress
	save()
}

// LocalProxyAddress() returns the host:port at which this lantern node listens
// for local (on computer) proxy connections, for example from the local web
// browser.
func LocalProxyAddress() string {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return config.LocalProxyAddress
}

func SetLocalProxyAddress(localProxyAddress string) {
	configMutex.Lock()
	defer configMutex.Unlock()
	config.LocalProxyAddress = localProxyAddress
	save()
}

/*
RemoteProxyAddress() returns the static host:port at which this lantern node
listens for remote proxy connections from other lantern nodes.

This lantern node may also listen on additional addresses based on the P2P
NAT traversal logic.
*/
func RemoteProxyAddress() string {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return config.RemoteProxyAddress
}

func SetRemoteProxyAddress(remoteProxyAddress string) {
	configMutex.Lock()
	defer configMutex.Unlock()
	config.RemoteProxyAddress = remoteProxyAddress
	save()
}

/*
StaticProxyAddresses() returns the host:port combinations at which this lantern
instance can find proxies with static ips (helpful for bootstrapping).

An empty value means that there is no static proxy known.
*/
func StaticProxyAddresses() []string {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return config.StaticProxyAddresses
}

func SetStaticProxyAddresses(staticProxyAddresses []string) {
	configMutex.Lock()
	defer configMutex.Unlock()
	config.StaticProxyAddresses = staticProxyAddresses
	save()
}

// UIAddress() returns the host:port
func UIAddress() string {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return config.UIAddress
}

func SetUIAddress(uiAddress string) {
	configMutex.Lock()
	defer configMutex.Unlock()
	config.UIAddress = uiAddress
	save()
}

// Email() returns the email address under which this lantern instance is
// running.  Server instances have a blank email address.
func Email() string {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return config.Email
}

func SetEmail(email string) {
	configMutex.Lock()
	defer configMutex.Unlock()
	config.Email = email
	save()
}

// configData defines the data structure of the config data as it is saved on
// disk (in JSON).
type configData struct {
	ParentAddress        string   // the host:port of our parent node (or "" if we're a root)
	SignalingAddress     string   // the host:port at which we will listen for signaling connections from our children
	LocalProxyAddress    string   // the host:port at which we will listen for local proxy connections (e.g. from the browser)
	RemoteProxyAddress   string   // the host:port at which we will listen for remote proxy connections from peers
	StaticProxyAddresses []string // array of host:port for known static proxies
	UIAddress            string   // the host:port at which the UI's backend listens
	Email                string   // the email address of the user under which this node is running (leave "" for server nodes)
}

var (
	// ConfigDir is the directory where lantern's configuration files are stored
	ConfigDir = determineConfigDir()
	// configFile is the location of our config file
	configFile = ConfigDir + "/config.json"
	// config is initialized with a set of default values
	config = &configData{
		ParentAddress:        "",
		SignalingAddress:     ":16100",
		LocalProxyAddress:    "127.0.0.1:8080",
		RemoteProxyAddress:   ":16200",
		StaticProxyAddresses: []string{},
		UIAddress:            "127.0.0.1:16300"}
	// configMutex is used to synchronize concurrent reads/writes of config properties
	configMutex sync.RWMutex
	// saveChannel is used to queue up requests to save the config back to disk
	saveChannel = make(chan configData, 100)
)

func init() {
	go saver()
	loadConfig()
}

// determineConfigDir() determines where to load the config by checking the
// command line and defaulting to ~/.lantern.
func determineConfigDir() string {
	flag.Parse()
	if flag.NArg() > 0 {
		return flag.Arg(0)
	} else {
		usr, err := user.Current()
		if err != nil {
			log.Fatal(err)
		}
		return usr.HomeDir + "/.lantern"
	}
}

// loadConfig() loads the configuration file from the ConfigDir.  If no file
// is present, a file will be created based on a default configuration.
func loadConfig() {
	if configFileData, err := ioutil.ReadFile(configFile); err != nil {
		log.Printf("Unable to find existing %s, keeping defaults: %s", configFile, err)
	} else {
		log.Printf("Initializing configuration from: %s", configFile)
		if err := json.Unmarshal(configFileData, config); err != nil {
			log.Printf("Unable to load config from %s, keeping defaults %s", configFile, err)
		}
	}
	save()
}

// save() requests a save by the saver goroutine.
func save() {
	saveChannel <- *config
}

// saver(), meant to be run as a goroutine, saves the config file after updates.
func saver() {
	select {
	case updated := <-saveChannel:
		log.Print("Saving config")
		configFileData, err := json.MarshalIndent(updated, "", "   ")
		if err != nil {
			log.Printf("Unable to marshal config to json: %s", err)
		} else {
			if err := ioutil.WriteFile(configFile, configFileData, 0600); err != nil {
				log.Printf("Unable to save config to %s: %s", configFile, err)
			}
			log.Printf("Config saved to %s", configFile)
		}
	}
}
