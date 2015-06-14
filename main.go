package main

import (
	"fmt"
	"github.com/go-martini/martini"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"time"
)

func main() {
	os.Exit(realMain())
}

func updateIPTables(ipList []string) {
	exec.Command("iptables", "-N", "GOTRUST").Run()
	exec.Command("iptables", "-F", "GOTRUST").Run()

	// Allow Serf to maintain membership
	exec.Command("iptables", "-A", "GOTRUST", "-p", "udp", "--destination-port", "7946", "-j", "ACCEPT").Run()

	// List all the available friends
	for _, ip := range ipList {
		exec.Command("iptables", "-A", "GOTRUST", "--src", ip, "-j", "ACCEPT").Run()
	}

	// Reject all the others by default
	exec.Command("iptables", "-A", "GOTRUST", "-j", "REJECT").Run()
}

func iptablesSynchronizer(ipLists chan []string) {
	currentList := make([]string, 0)
	for {
		newList := <-ipLists
		sort.Strings(newList)

		if !reflect.DeepEqual(currentList, newList) {
			fmt.Println("Updating IPTables with the new IP list")
			currentList = newList
			updateIPTables(currentList)
		}
	}
}

func friendListMaintainance(friendNotifications chan string) {
	ipTables := make(chan []string)
	go iptablesSynchronizer(ipTables)

	friends := make(map[string]time.Time)
	ticker := time.Tick(1 * time.Second)
	for {
		changed := false

		select {
		case friend := <-friendNotifications:
			fmt.Printf("Friend notification: %s\n", friend)
			changed = true
			friends[friend] = time.Now()
		case <-ticker:
		}

		threshold := time.Now().Add(-1 * time.Minute)
		for ip, time := range friends {
			if time.Before(threshold) {
				changed = true
				fmt.Printf("%s has expired, and being removed from the friend list\n", ip)
				delete(friends, ip)
			}
		}

		if changed {
			if len(friends) > 0 {
				fmt.Println("Friend list has changed, and it currently is:")
				for ip := range friends {
					fmt.Printf("  - %s\n", ip)
				}
			} else {
				fmt.Println("Friend list became empty")
			}

			ipAddresses := make([]string, 0)
			for k, _ := range friends {
				ipAddresses = append(ipAddresses, k)
			}
			ipTables <- ipAddresses
		}
	}
}

func realMain() int {
	log.SetOutput(ioutil.Discard)

	m := martini.Classic()

	m.Get("/", func() string {
		return "Cluster IPTables maintainance service"
	})

	newFriends := make(chan string)
	go friendListMaintainance(newFriends)
	m.Post("/friend", func(req *http.Request, log *log.Logger) string {
		ip := req.FormValue("ip")
		newFriends <- ip
		return fmt.Sprintf("{ result: 0, ip: \"%s\" } ", ip)
	})

	log.Print("Cluster IPTables maintainance service is listening on 127.0.0.1:17395")
	m.RunOnAddr("127.0.0.1:17395")

	return 0
}
