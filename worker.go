package main

import (
	"log"
	"github.com/streadway/amqp"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"io/ioutil"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"io"
	"net/url"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

type jsonResponse struct {
	DriveID string   `json:"drive_id"`
	Images  []string `json:"images"`
}

func main() {
	conn, err := amqp.Dial("amqp://admin:admin@54.146.153.139")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"task_queue", // name
		true,         // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	failOnError(err, "Failed to declare a queue")

	err = ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	failOnError(err, "Failed to set QoS")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	//GET CLIENT
	ctx := context.Background()
	b, err := ioutil.ReadFile("/home/ubuntu/go/src/github.com/huyhvq/learn/client_secret.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved credentials

	config, err := google.ConfigFromJSON(b, drive.DriveScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	client := getClient(ctx, config)

	srv, err := drive.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve drive Client %v", err)
	}

	forever := make(chan bool)
	go func() {
		for d := range msgs {
			var imgs jsonResponse
			byt := []byte(d.Body)
			json.Unmarshal(byt, &imgs)
			for index, img := range imgs.Images {
				s := fmt.Sprintf("%03d.jpg", index)
				upload(srv, img, s, imgs.DriveID)
			}
			fmt.Println("Done")
			d.Ack(false)
		}
	}()
	<-forever
	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
}

// getClient uses a Context and Config to retrieve a Token
// then generate a Client. It returns the generated Client.
func getClient(ctx context.Context, config *oauth2.Config) *http.Client {
	tok, err := tokenFromFile("/home/ubuntu/go/src/github.com/huyhvq/learn/credential.json")
	if err != nil {
		log.Fatalf("Unable to get path to cached credential file. %v", err)
	}

	return config.Client(ctx, tok)
}

// tokenFromFile retrieves a Token from a given file path.
// It returns the retrieved Token and any read error encountered.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	t := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(t)
	defer f.Close()
	return t, err
}

func upload(srv *drive.Service, src string, filename string, parent string) {
	_, err := url.ParseRequestURI(src)
	if err != nil {
		log.Println("Can't open url - go to next URL")
	} else {
		resp, err := http.Get(src)
		if err != nil {
			log.Println("Can't open url %v", err)
		}
		defer resp.Body.Close()
		file, err := ioutil.TempFile(os.TempDir(), "dpmanga_")
		if err != nil {
			log.Fatalf("Can't create tmp file %v", err)
		}
		defer file.Close()
		io.Copy(file, resp.Body)
		goFile, err := os.Open(file.Name())
		if err != nil {
			log.Fatalf("Can't open tmp file %v", err)
		}
		fld, _ := srv.Files.Create(&drive.File{Name: filename, Parents: []string{parent}}).Media(goFile).Do()

		fmt.Printf("[Upload SUCCESS !!!] - Folder: %s - File Id: %s \n", parent, fld.Id)

		os.Remove(file.Name())
		fmt.Println("remove tmp file success!!")
	}
}
