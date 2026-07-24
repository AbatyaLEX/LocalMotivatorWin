package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
	"google.golang.org/genai"
)


func main(){

	ctx, cancel := CreateContext()
	defer cancel()
	client := InitClient(ctx)
	contents := []*genai.Content{
		genai.NewContentFromText("Duck", genai.RoleUser),
	}
	response, err := client.Models.GenerateContent(ctx, "gemini-3.6-flash", contents, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(response.Text())
	OwnTicker()
}

func OwnTicker(){
	ticker  :=  time.NewTicker(1000 *  time.Millisecond)
	done :=  make(chan struct{})
	go  func()  {
		for {
			select{
			case  <-done:
				return
			case  t :=  <-ticker.C:
				fmt.Println("Tick at", t)
			}
		}
	}()

	time.Sleep(5000 * time.Millisecond)
	ticker.Stop()
	close(done)
	fmt.Println("Stop")
}

func CreateContext()(context.Context, context.CancelFunc){
	baseCtx := context.Background()
	ctx, cancel := context.WithTimeout(baseCtx, 10*time.Second)
	return ctx, cancel
}

func  InitClient(ctx context.Context)*genai.Client{
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("Gemini_API_Key"),
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Fatal(err)
	}
	return client
}