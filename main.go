package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/mux"
	strip "github.com/grokify/html-strip-tags-go"
	"github.com/kelseyhightower/envconfig"
	"github.com/mmcdole/gofeed"
	"github.com/robfig/cron"
)

const feedURLLeagueNews = "https://na.leagueoflegends.com/en/rss.xml"

var (
	conf      config
	GitCommit string
)

type config struct {
	DiscordToken   string `split_words:"true" required:"true"`
	DiscordChannel string `split_words:"true" required:"true"`
}

func init() {
	// Environment variable
	err := envconfig.Process("RYZE", &conf)
	if err != nil {
		log.Fatal(err.Error())
	}

	version := flag.Bool("v", false, "Print the version of the application")
	notifNewsLeagueOff := flag.Int("notif-news-off", 0, "Send to discord N last news from League Of Legend Official site")
	flag.Parse()

	if *version {
		fmt.Printf("Commit: %s\n", GitCommit)
		os.Exit(0)
	}

	if *notifNewsLeagueOff > 0 {
		notifyLastNNews(feedURLLeagueNews, *notifNewsLeagueOff)
		os.Exit(0)
	}
}

func main() {
	log.Println("Ryze starting ...")
	c := cron.New()
	c.AddFunc("0 * * * * *", func() {
		cronRSSNews(feedURLLeagueNews)
	})
	c.Start()

	router := mux.NewRouter()
	router.HandleFunc("/_health_check", healthCheckHandler).Methods("GET")
	log.Fatal(http.ListenAndServe(":8000", router))
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode("OK")
}

func cronRSSNews(feedURL string) {
	// Get last News
	var newsToSend []*feedItem
	lastNews := getLastNews(feedURL, 10)

	// Send to Discord only if there is new item
	now := time.Now()
	for _, new := range lastNews {
		if now.Sub(*new.PublishedParsed) < time.Second*60 {
			newsToSend = append(newsToSend, new)
		}
	}

	if len(newsToSend) > 0 {
		notifyDiscord(newsToSend)
	}
}

func notifyLastNNews(feedURL string, n int) {
	news := getLastNews(feedURL, n)
	notifyDiscord(news)
}

func getLastNews(feedURL string, n int) []*feedItem {
	fp := gofeed.NewParser()
	feed, _ := fp.ParseURL(feedURL)
	var lastNews []*feedItem

	for i := 0; i <= n-1; i++ {
		new := &feedItem{
			Source:          feed.Title,
			Title:           feed.Items[i].Title,
			Description:     truncDescription(sanitizeHTML(feed.Items[i].Description)),
			Link:            feed.Items[i].Link,
			PublishedParsed: feed.Items[i].PublishedParsed,
		}
		lastNews = append(lastNews, new)
	}

	return lastNews
}

func sanitizeHTML(s string) string {
	sanitized := strip.StripTags(s)
	sanitized = html.UnescapeString(sanitized)

	return sanitized
}

func truncDescription(s string) string {
	// If there is '\n' it's probably a big description
	// So we need to truncate it
	if strings.ContainsAny(s, "\n") {
		trunc := strings.SplitN(s, "\n", 2)[0]
		return fmt.Sprintf("%s ...\n", trunc)
	}

	return s
}

type feedItem struct {
	Source          string
	Title           string
	Description     string
	Link            string
	PublishedParsed *time.Time
}

func newDiscordSession() *discordgo.Session {
	// Create a new Discord session using the provided bot token.
	session, err := discordgo.New("Bot " + conf.DiscordToken)
	if err != nil {
		log.Fatal("error creating Discord session,", err)
	}

	err = session.Open()
	if err != nil {
		log.Fatal(err)
	}

	return session
}

func notifyDiscord(news []*feedItem) {
	// Reverse news (to have the more recent at the end of the slice)
	for left, right := 0, len(news)-1; left < right; left, right = left+1, right-1 {
		news[left], news[right] = news[right], news[left]
	}

	session := newDiscordSession()
	defer session.Close()

	for _, new := range news {
		message := formatRSSDiscordMessage(new)
		_, err := session.ChannelMessageSendEmbed(conf.DiscordChannel, message)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func formatRSSDiscordMessage(new *feedItem) *discordgo.MessageEmbed {
	msgFieldTitle := &discordgo.MessageEmbedField{
		Name:   "Title",
		Value:  new.Title,
		Inline: true,
	}

	msgFieldLink := &discordgo.MessageEmbedField{
		Name:   "Link",
		Value:  new.Link,
		Inline: true,
	}

	msgFields := []*discordgo.MessageEmbedField{msgFieldLink, msgFieldTitle}

	message := &discordgo.MessageEmbed{
		Title:       new.Source,
		Description: new.Description,
		Fields:      msgFields,
	}

	return message
}
