package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/vmware/govmomi"

	"net/url"
	"os"

	"math/rand"
	"regexp"

	"gopkg.in/yaml.v2"
)

type ConfigFile struct {
	Token string `yaml:"token"`
}

type machineGroup struct { //The format for each category
	name     string
	machines []string
}

type messageLog struct {
	sourceSession []*discordgo.Session
	sourceMessage []*discordgo.MessageCreate
	queueTag      []string
}

var Token string
var messageHandling messageLog
var standalone = machineGroup{"standalone", []string{"Shipping", ""}}                                                                        //put all standalone machines here
var lab1 = machineGroup{"lab1", []string{"WEB01", "WS01", "DOCS", "DEV", "DB01", "CORP-WEB01", "ADMIN-DB", "HERBERT-PC", "WEB-DEV", "DC01"}} //put all lab1 machines here

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
		Host:   "",
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

func queue(s *discordgo.Session, m *discordgo.MessageCreate) {
	//messageHandling.messages.append(m.)
	if m.Author.ID != "963614108001902642" {
		uniqueTag := strconv.Itoa(rand.Intn(1000000000000))                                           //Big random number to use as an identifier
		messageHandling.sourceMessage = append(messageHandling.sourceMessage, m)                      //Adding to the queue
		messageHandling.sourceSession = append(messageHandling.sourceSession, s)                      //Adding to the queue
		messageHandling.queueTag = append(messageHandling.queueTag, uniqueTag)                        //Adding to the queue
		if _, currentPosition := contains(messageHandling.queueTag, uniqueTag); currentPosition > 0 { //checks if the slice has out ID, and if our ID is > 0 in the queue (meaning theres a line)
			for currentPosition > 0 { // scuffed while loop
				time.Sleep(time.Second * 1) //Keep waiting until the current position is the next one
				_, currentPosition = contains(messageHandling.queueTag, uniqueTag)
			}
			action(messageHandling.sourceSession[0], messageHandling.sourceMessage[0])
		} else {
			time.Sleep(time.Second * 1) //Wait an extra 3 second before doing anything
			action(messageHandling.sourceSession[0], messageHandling.sourceMessage[0])
		}

		messageHandling.sourceSession = removeSession(messageHandling.sourceSession, 0) //pop the stuff from queue
		messageHandling.sourceMessage = removeMessage(messageHandling.sourceMessage, 0)
		messageHandling.queueTag = removeTag(messageHandling.queueTag, 0)

	}
}

func removeSession(slice []*discordgo.Session, s int) []*discordgo.Session { //remove first item from  of source message
	return append(slice[:s], slice[s+1:]...)
}
func removeMessage(slice []*discordgo.MessageCreate, s int) []*discordgo.MessageCreate {
	return append(slice[:s], slice[s+1:]...)
}
func removeTag(slice []string, s int) []string {
	return append(slice[:s], slice[s+1:]...)
}
func action(s *discordgo.Session, m *discordgo.MessageCreate) { //Bot does stuff

	if m.Content == "!!bruh" { //debug send message
		s.ChannelMessageSend(m.ChannelID, "bruh")
	}

	if m.Content == "!!login status" { //see if i can log into vsphere
		s.ChannelMessageSend(m.ChannelID, strconv.FormatBool(auth()))
	}
	/*
		defer func() {
			if panicInfo := recover(); panicInfo != nil {
				time.Sleep(1)
				s.ChannelMessageSend(m.ChannelID, "Command brokey, try again? Example: !!reset localhost lab1 or !!reset localhost standalone")
			}
		}()
	*/
	if strings.HasPrefix(m.Content, "!!reset") {
		var wholeString string
		re := regexp.MustCompile(`!!reset .{0,} .{0,}`)

		matches := re.FindStringSubmatch(m.Content)
		wholeString = matches[0]
		hostname := strings.ToUpper(strings.Split(wholeString, " ")[1]) //uppercase for easier handling
		category := strings.ToLower(strings.Split(wholeString, " ")[2]) //lowercase for easier handling. This is the lab/standalone
		//check if it is a valid reset
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Reset request in %s from %s", category, m.Author.Username))
		time.Sleep(1 * time.Second)
		s.ChannelMessageSend(m.ChannelID, "Checking if the values are valid ...")
		time.Sleep(1 * time.Second)
		if category == "lab1" {
			validMachine(lab1, hostname, category, s, m)
		} else {
			time.Sleep(1 * time.Second)
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Invalid category"))
		}
	}
}

func validMachine(group machineGroup, nameToCheck string, category string, s *discordgo.Session, m *discordgo.MessageCreate) { //Compared a nameToCheck to the rest of the machines in the group to see if it exists
	for _, x := range group.machines {
		if x == nameToCheck {
			time.Sleep(1 * time.Second)
			if valid, domainName := isDomain(group, nameToCheck); valid { //Since isDomain returns two values, create temp var to take the bool valu and check that one
				resetCheck(domainName, category, s, m)
				//s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Resetting the %s from %s", domainName, category))
			} else {
				resetCheck(nameToCheck, category, s, m)
				//s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Resetting %s from %s", nameToCheck, category))
			}
			return
		}
	}
	time.Sleep(1 * time.Second)
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Values are invalid"))

}
func resetCheck(name string, category string, s *discordgo.Session, m *discordgo.MessageCreate) { //funky printing
	timeLeft := 20  //default 20 seconds
	percentage := 0 //default empty percentage

	voteMessageSent, _ := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Vote to reset %s from %s \nReact with :green_circle: to vote yes, :red_circle: for no", name, category))
	timeLeftMessage, _ := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Time left: %d", timeLeft))
	progressBarMessage, _ := s.ChannelMessageSend(m.ChannelID, "Voting Percent: ")
	fmt.Println("message channel id: ", m.ChannelID, "\nvotemessagesent channel id:", voteMessageSent.ChannelID) //id is valid
	fmt.Println("voting message message id: ", voteMessageSent.ID)                                               //messageid is null
	s.MessageReactionAdd(voteMessageSent.ChannelID, voteMessageSent.ID, "\U0001f7e2")                            //reacting to message
	fmt.Println("debugging here")
	s.MessageReactionAdd(voteMessageSent.ChannelID, voteMessageSent.ID, "\U0001f534") //can get this by print([f"0x{ord(c):08x}" for c in "🔴"]) in python
	for timeLeft > 0 {
		timeLeft -= 1 //every second, lower the timer
		asciiBar := ""
		counter := 2                                                                                                        //counter to fill up the percentage bar via division
		s.ChannelMessageEdit(timeLeftMessage.ChannelID, timeLeftMessage.ID, fmt.Sprintf("Time left: %d", timeLeft))         //edit the timer
		yesReactionUsers, _ := s.MessageReactions(voteMessageSent.ChannelID, voteMessageSent.ID, "\U0001f7e2", 100, "", "") //counting greens
		noReactionUsers, _ := s.MessageReactions(voteMessageSent.ChannelID, voteMessageSent.ID, "\U0001f534", 100, "", "")  //counting reds                                                                                       //debug
		totalReactions := len(yesReactionUsers) + len(noReactionUsers)
		percentage = (len(yesReactionUsers) * 100 / totalReactions)
		nullPercentage := 100 - percentage
		for counter < percentage {
			asciiBar += "■"
			counter += 2
		}
		counter = 2 //reset counter back to base
		for counter < nullPercentage {
			asciiBar += "□"
			counter += 2
		}
		asciiBarTail := fmt.Sprintf(" %d", percentage)
		s.ChannelMessageEdit(progressBarMessage.ChannelID, progressBarMessage.ID, "Voting Percent: "+asciiBar+asciiBarTail+"%")
	}
	if percentage >= 50 { //timer is done, voting time
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Resetting %s from %s", name, category))
	} else {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Not enough votes to reset %s from %s", name, category))
	}

}
func isDomain(group machineGroup, nameToCheck string) (bool, string) { //Check if the machine is in a domain. This assumes that the machine is already valid within the group
	domainMachines1 := []string{"ADMIN-DB", "WEB-DEV", "DC01", "HERBERT-PC"}
	if doesContain, _ := contains(domainMachines1, nameToCheck); group.name == "lab1" && doesContain {
		return true, "the Admin Subnet"
	}
	return false, "false"
}

func contains(stringArray []string, stringToCheck string) (bool, int) { //basic function to check if string array contains a certain string. its like validMachien but more broken down
	index := 0
	for _, item := range stringArray {
		if item == stringToCheck {
			return true, index
		}
		index += 1
	}
	return false, -1
}

func newVoteEmbed() *discordgo.MessageEmbed {
	var voteEmbed *discordgo.MessageEmbed
	voteEmbed = &discordgo.MessageEmbed{
		Title:       "Voting Status",
		Description: "Time left: ",
		Color:       16721189,
		Fields: []*discordgo.MessageEmbedField
	}
	return voteEmbed
}
func main() {
	auth()

	var c ConfigFile
	var dg *discordgo.Session

	config := c.getConfigFile()
	Token = config.Token
	/*
		commandGuild = config.Guild
		commandChannel = config.Channel
	*/

	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("[!] Error starting a Bot session", err)
		return
	}
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(queue)

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
