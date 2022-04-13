package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/vmware/govmomi"

	"net/url"
	"os"

	"gopkg.in/yaml.v2"
)

type ConfigFile struct {
	Token   string `yaml:"token"`
	Guild   string `yaml:"guild"`
	Channel string `yaml:"channel"`
}

var Token string
var commandGuild string
var commandChannel string
var commandChannelID string
var generalChannelID string

func (c *ConfigFile) getConfigFile() *ConfigFile {

	yamlFile, err := ioutil.ReadFile("config.yml")
	if err == nil {
		err = yaml.Unmarshal(yamlFile, c)
		if err == nil {
			fmt.Println("[+] Config file loaded")
			return c
		} else {
			fmt.Println("[-] Bad config.yml format")
		}
	} else {
		fmt.Println("[-] Cannot read the config.yml file")
	}
	return nil
}

func auth() bool { //Authenticate to vsphere

	const (
		user     = "" //Declare creds
		password = ""
	)
	u := &url.URL{ //url to send post req to vsphere (aka login)
		Scheme: "https",
		Host:   "vsphere.telcolab.xyz",
		Path:   "/sdk/",
	}
	ctx := context.Background()                    //idk tbh copy and paste
	u.User = url.UserPassword(user, password)      //login object
	client, err := govmomi.NewClient(ctx, u, true) //attempt the login
	if err != nil {
		fmt.Fprintf(os.Stderr, "Login to vsphere failed, %v", err)
		os.Exit(1)
	}
	return client.IsVC()
}

func message(s *discordgo.Session, m *discordgo.MessageCreate) {

	if m.Content == "!!bruh" {
		s.ChannelMessageSend(m.ChannelID, "bruh")
	}
}
func main() {
	auth()

	var c ConfigFile
	var dg *discordgo.Session

	config := c.getConfigFile()
	Token = config.Token
	commandGuild = config.Guild
	commandChannel = config.Channel

	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("[!] Error starting a Bot session", err)
		return
	}
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(message)

	// In this example, we only care about receiving message events.
	dg.Identify.Intents = discordgo.IntentsGuildMessages

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
	dg.Close()
	//<-make(chan struct{})
	//return

}
