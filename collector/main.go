package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"golang.org/x/crypto/ssh"
)

type Machine struct {
	Id       int    // VM ID on ESXi
	Name     string // VM Name on ESXi
	NodeName string // ESXi Node Name
}
type Machines []Machine

type esxiNode struct {
	Address      string
	User         string
	IdentityFile string `toml:"identity_file"`
}
type esxiNodes map[string]esxiNode

// Get SSH Nodes from hosts.toml
func loadAllEsxiNodes() esxiNodes {
	esxiNodeConfPath := "hosts.toml"

	content, err := ioutil.ReadFile(esxiNodeConfPath)
	if err != nil {
		log.Fatalln(err)
	}

	var nodes esxiNodes
	if _, err := toml.Decode(string(content), &nodes); err != nil {
		log.Fatalln(err)
	}
	/* Debug
	for key,value := range esxiNodes {
		println(key, "=>", value.Name, value.Address, value.User)
	} */

	return nodes
}

// Get VM info from ESXi via SSH
func execCommandSsh(ip string, config *ssh.ClientConfig, command string) (bytes.Buffer, error) {
	var buf bytes.Buffer

	conn, err := ssh.Dial("tcp", ip+":22", config)
	if err != nil {
		return buf, err
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return buf, err
	}
	defer session.Close()

	session.Stdout = &buf
	if err := session.Run(command); err != nil {
		return buf, err
	}

	return buf, nil
}

// Parse command result
func parseResultAllVms(buf bytes.Buffer) Machines {
	r := regexp.MustCompile(`^\d.+`)
	var vms Machines
	for {
		st, err := buf.ReadString('\n')
		if err != nil {
			return vms
		}
		if !r.MatchString(st) {
			continue
		}

		slice := strings.Split(st, "   ")
		slice0, err := strconv.Atoi(slice[0])
		if err != nil {
			slice0 = -1
		}
		vms = append(vms, Machine{
			Id:   slice0,
			Name: strings.TrimSpace(slice[1]),
		})
	}
}

// Get All VM Name and VM Id
func GetAllVmIdName() Machines {
	allVm := Machines{}
	for nodeName, nodeInfo := range loadAllEsxiNodes() {
		buf, err := ioutil.ReadFile(nodeInfo.IdentityFile)
		if err != nil {
			log.Fatalln(err)
		}
		key, err := ssh.ParsePrivateKey(buf)
		if err != nil {
			log.Fatalln(err)
		}

		// ssh connect
		nodeAddr := nodeInfo.Address
		config := &ssh.ClientConfig{
			User:            nodeInfo.User,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(key),
			},
		}
		b, err := execCommandSsh(nodeAddr, config, "vim-cmd vmsvc/getallvms")
		if err != nil {
			log.Println(err.Error())
		}

		// update vm list
		for _, vm := range parseResultAllVms(b) {
			allVm = append(allVm, Machine{
				Id:       vm.Id,
				Name:     vm.Name,
				NodeName: nodeName,
			})
		}
	}
	return allVm
}

func main() {
	for _, vm := range GetAllVmIdName() {
		log.Println(vm.NodeName, vm.Id, vm.Name)
		// if vm.Name == hostname {
		// 	return GetVmIp(vm)
		// }
	}
}
