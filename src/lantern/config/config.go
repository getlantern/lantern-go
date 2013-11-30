/*
Encapsulates the configuration for the Lantern system, which is backed by a
config.json file stored on the file system.

The config.json is found in the [ConfigDir].

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

// Defines the data structure of the config data as it is saved on disk (in JSON)
type configData struct {
	ParentAddress      string // the host:port of our parent node (or "" if we're a root)
	SignalingAddress   string // the host:port at which we will listen for signaling connections from our children
	LocalProxyAddress  string // the host:port at which we will listen for local proxy connections (e.g. from the browser)
	RemoteProxyAddress string // the host:port at which we will listen for remote proxy connections from peers
	UIAddress          string // the host:port at which the UI's backend listens
	Email              string // the email address of the user under which this node is running (leave "" for server nodes)
}

var (
	ConfigDir  = determineConfigDir()
	configFile = ConfigDir + "/config.json"
	config     = &configData{
		ParentAddress:      "",
		SignalingAddress:   ":16100",
		LocalProxyAddress:  "127.0.0.1:8080",
		RemoteProxyAddress: ":16200",
		UIAddress:          "127.0.0.1:16300"}
	configMutex sync.RWMutex
	saveChannel = make(chan configData, 100)
)

func ParentAddress() string {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return config.ParentAddress
}

func SetParentAddress(parentAddress string) {
	configMutex.Lock()
	defer configMutex.Unlock()
	config.ParentAddress = parentAddress
	save()
}

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

func init() {
	go saver()
	loadConfig()
}

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

// Request a save by the saver goroutine
func save() {
	saveChannel <- *config
}

// Goroutine for saving the config after updates
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
		}
	}
}
