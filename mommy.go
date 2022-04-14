package main

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"

	"os"

	"math/rand"
	"regexp"

	"gopkg.in/yaml.v2"
)

type ConfigFile struct {
	Token string `yaml:"token"`
}

type machineGroup struct { //The format for each category
	name        string
	machines    []string
	domainBoxes []domainMachines //A lab will use this. Its an array of a struct. the struct will have a domain name and an array that contains associated machines in the domain
}
type domainMachines struct {
	domainName  string
	machineName []string
}
type messageLog struct {
	sourceSession []*discordgo.Session
	sourceMessage []*discordgo.MessageCreate
	queueTag      []string
}

var Token string
var messageHandling messageLog
var standalone = machineGroup{
	"standalone",
	[]string{"Shipping", ""},
	nil, //has no domainboxes
}                        //put all standalone machines here
var lab1 = machineGroup{ //put all lab1 machines here
	"lab1",
	[]string{"WEB01", "WS01", "DOCS", "DEV", "DB01", "CORP-WEB01", "ADMIN-DB", "HERBERT-PC", "WEB-DEV", "DC01"}, //all the machines
	[]domainMachines{
		{
			"The Admin Subnet",
			[]string{".110 ADMIN-DB", ".115 HERBERT-PC", ".120 WEB-DEV", ".105 DC01"}, //the domain joined ones (shitty naming context, dont do this from now on)
		},
	},
}

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
	/*
		if m.Content == "!!login status" { //see if i can log into vsphere
			s.ChannelMessageSend(m.ChannelID, strconv.FormatBool(auth()))
		}
	*/
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			time.Sleep(1 * time.Second)
			s.ChannelMessageSend(m.ChannelID, "Command brokey, try again? Example: !!reset localhost lab1 or !!reset localhost standalone")
		}
	}()

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
			s.ChannelMessageSend(m.ChannelID, "Invalid category")
		}
	}
}

func validMachine(group machineGroup, nameToCheck string, category string, s *discordgo.Session, m *discordgo.MessageCreate) { //Compared a nameToCheck to the rest of the machines in the group to see if it exists
	for _, x := range group.machines {
		if x == nameToCheck {
			time.Sleep(1 * time.Second)
			if valid, domainName := isDomain(group, nameToCheck); valid { //Since isDomain returns two values, create temp var to take the bool valu and check that one
				resetCheck(domainName, category, s, m, true)
				//s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Resetting the %s from %s", domainName, category))
			} else {
				resetCheck(nameToCheck, category, s, m, false)
				//s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Resetting %s from %s", nameToCheck, category))
			}
			return
		}
	}
	time.Sleep(1 * time.Second)
	s.ChannelMessageSend(m.ChannelID, "Values are invalid")

}
func resetCheck(name string, category string, s *discordgo.Session, m *discordgo.MessageCreate, isDomain bool) { //funky printing
	timeLeft := 15                       //default 15 seconds
	percentage := 0                     //default empty percentage
	votingEmbedObject := newVoteEmbed() //create the embed thing

	votingEmbedObject = modifyVoteEmbed(votingEmbedObject, timeLeft, category, name, "■■■■■■■■■■■■■■■■■■■■■■■■□□□□□□□□□□□□□□□□□□□□□□□□ 50%")
	embedMessage, _ := s.ChannelMessageSendEmbed(m.ChannelID, votingEmbedObject) //send the embed vote thing
	s.MessageReactionAdd(embedMessage.ChannelID, embedMessage.ID, "\U0001f7e2")  //react to the embed
	s.MessageReactionAdd(embedMessage.ChannelID, embedMessage.ID, "\U0001f534")  //react to the embed

	for timeLeft > 0 {
		time.Sleep(800 * time.Millisecond)
		timeLeft -= 1 //every second, lower the timer
		asciiBar := ""

		yesReactionUsers, _ := s.MessageReactions(embedMessage.ChannelID, embedMessage.ID, "\U0001f7e2", 100, "", "") //counting greens
		noReactionUsers, _ := s.MessageReactions(embedMessage.ChannelID, embedMessage.ID, "\U0001f534", 100, "", "")  //counting reds

		totalReactions := len(yesReactionUsers) + len(noReactionUsers)
		percentage = (len(yesReactionUsers) * 100 / totalReactions)
		nullPercentage := 100 - percentage

		//percentage = 50 //Debugging time values
		//nullPercentage := 50
		fillTicks := percentage / 2
		asciiBar = strings.Repeat("■", fillTicks)

		nullTicks := nullPercentage / 2
		nullBar := strings.Repeat("□", nullTicks)
		asciiBar += nullBar

		asciiBarTail := fmt.Sprintf(" %d", percentage)
		votingEmbedObject = modifyVoteEmbed(votingEmbedObject, timeLeft, category, name, asciiBar+asciiBarTail+"%")
		s.ChannelMessageEditEmbed(embedMessage.ChannelID, embedMessage.ID, votingEmbedObject)

	}
	if percentage >= 50 { //timer is done, voting time
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Resetting %s from %s", name, category))
		if isDomain {
			resetDomain(name, category)
		} else {
			resetMachine(name)
		}
		s.ChannelMessageSend(m.ChannelID, "Revert finished. Give it some time to breathe.")
	} else {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Not enough votes to reset %s from %s", name, category))
	}

}
func resetMachine(name string) {
	switch name { //an unfortunate solution to a shitty mistake. DONT PUT LAST OCTET IN NAME OF BOXES WHENCREATING, ITLL FUCKING ANNOYING THE SHIT OUTTA YOU
	case "WEB01":
		name = ".5 WEB01"
	case "WS01":
		name = ".10 WS01"
	case "DOCS":
		name = ".15 DOCS"
	case "DEV":
		name = ".20 DEV"
	case "DB01":
		name = ".25 DB01"
	case "CORP-WEB01":
		name = ".50 CORP-WEB01"
	}

	exec.Command(`/bin/bash`, `-c`, fmt.Sprintf(`env GOVC_URL='https://username:pass@vspherehost/sdk' /root/go/bin/govc snapshot.revert -k=true -vm.path="[truenas] %s/%s.vmx" -dc TelcoLabDataCenter "about to test"`, name, name)).Output()
	exec.Command(`/bin/bash`, `-c`, fmt.Sprintf(`env GOVC_URL=https://username:pass@vspherehost/sdk' /root/go/bin/govc vm.power -on -k=true -dc TelcoLabDataCenter -wait=true -vm.path="[truenas]%s/%s.vmx"`, name, name)).Output()
}
func resetDomain(domainName string, category string) {
	if domainName == "The Admin Subnet" && category == "lab1" {
		for _, name := range lab1.domainBoxes[0].machineName { //this one is the admin subnet
			exec.Command(`/bin/bash`, `-c`, fmt.Sprintf(`env GOVC_URL='https://username:pass@vspherehost/sdk' /root/go/bin/govc snapshot.revert -k=true -vm.path="[truenas] %s/%s.vmx" -dc TelcoLabDataCenter "about to test"`, name, name)).Output()
			exec.Command(`/bin/bash`, `-c`, fmt.Sprintf(`env GOVC_URL='https://username:pass@vspherehost/sdk' /root/go/bin/govc vm.power -on -k=true -dc TelcoLabDataCenter -wait=true -vm.path="[truenas]%s/%s.vmx"`, name, name)).Output()
		}
	}
}
func isDomain(group machineGroup, nameToCheck string) (bool, string) { //Check if the machine is in a domain. This assumes that the machine is already valid within the group
	domainMachines1 := []string{"ADMIN-DB", "WEB-DEV", "DC01", "HERBERT-PC"}
	if doesContain, _ := contains(domainMachines1, nameToCheck); group.name == "lab1" && doesContain {
		return true, "The Admin Subnet"
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
		Description: "Time left", //show time left here
		Color:       16721189,
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{
				Name:   "Category: ", //is it lab1, lab2, standalone, etc.
				Value:  "Name: ",     //Box title or subnet
				Inline: false,
			},
			&discordgo.MessageEmbedField{
				Name:   "Voting Percentage",
				Value:  "",
				Inline: false,
			},
		},
	}
	return voteEmbed
}

func modifyVoteEmbed(embedObject *discordgo.MessageEmbed, timeLeft int, category string, name string, percentageBar string) *discordgo.MessageEmbed {
	embedObject.Description = fmt.Sprintf("Time left: %d", timeLeft)
	embedObject.Fields[0].Name = category
	embedObject.Fields[0].Value = "Name: " + name
	embedObject.Fields[1].Value = percentageBar
	return embedObject
}

func main() {
	//auth()

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
