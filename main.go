package main
import "time"
import "fmt"

func main(){
	ticker  :=  time.NewTicker(1000 *  time.Millisecond)
	done :=  make(chan bool)
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
	done <- true
	fmt.Println("Stop")
}