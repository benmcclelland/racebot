package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/benmcclelland/gogrove"
)

var (
	lightSensor = gogrove.PortA0
	button2     = gogrove.PortD2
	button1     = gogrove.PortD3
	relay       = gogrove.PortD5
	led1        = gogrove.PortD6
	session     *gogrove.Session
	lcd         *gogrove.LCD

	tracklengthInInches float64 = 162
	// display demo
	// tracklengthInInches float64 = 23
	mphConv = (tracklengthInInches * secInHour) / inchesInMile
)

const (
	threshold    float64 = 300
	inchesInMile float64 = 63360
	secInHour    float64 = 3600
)

func init() {
	var err error
	session, err = gogrove.New()
	if err != nil {
		panic(err)
	}
	lcd, err = gogrove.NewLCD()
	if err != nil {
		panic(err)
	}

	session.SetPortMode(lightSensor, gogrove.ModeInput)
	session.SetPortMode(button1, gogrove.ModeInput)
	session.SetPortMode(button2, gogrove.ModeInput)
	session.SetPortMode(led1, gogrove.ModeOutput)
	session.SetPortMode(relay, gogrove.ModeOutput)
}

func cleanup() {
	session.TurnOff(relay)
	session.TurnOff(led1)
	lcd.SetRGB(0, 0, 0)
	lcd.SetText("")
}

func showErr(err error) {
	log.Printf("err: %v", err)
	session.TurnOff(relay)
	session.TurnOff(led1)
	lcd.SetRGB(255, 255, 0)
	lcd.SetText(fmt.Sprintf("Err:\n%v", err))
	time.Sleep(2 * time.Second)
}

func startingGateReady() {
	session.TurnOn(relay)
	lcd.SetText("READY")
	lcd.SetRGB(0, 0, 255)
}

func waitForButtonPress() error {
	for {
		bpress, err := session.DigitalRead(button1)
		if err != nil {
			continue
			//return fmt.Errorf("check button: %v", err)
		}
		if bpress == 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func waitForLightBreak(ambient uint16) error {
	for {
		lightval, err := session.AnalogRead(lightSensor)
		if err != nil {
			//log.Printf("ERR: check light sensor: %v", err)
			continue
		}
		//log.Printf("ambient: %v, lightval: %v, diff: %v", ambient, lightval, lightval-ambient)
		if math.Abs(float64(ambient)-float64(lightval)) > float64(threshold) {
			break
		}
	}
	return nil
}

func runRace() (time.Duration, error) {
	lcd.SetText("Running!!!")
	lcd.SetRGB(0, 255, 0)
	ambient, err := session.AnalogRead(lightSensor)
	if err != nil {
		return 0, fmt.Errorf("check light sensor: %v", err)
	}
	session.TurnOn(led1)
	session.TurnOff(relay)

	start := time.Now()
	err = waitForLightBreak(ambient)
	if err != nil {
		return 0, err
	}
	session.TurnOff(led1)
	return time.Since(start), nil
}

func displayResults(et time.Duration) {
	mph := mphConv / et.Seconds()
	lcd.SetText(fmt.Sprintf("elapsed: %.3f\nMPH: %.3f", et.Seconds(), mph))
	lcd.SetRGB(255, 0, 0)
}

func racebot() {
	for {
		startingGateReady()
		time.Sleep(500 * time.Millisecond)
		err := waitForButtonPress()
		if err != nil {
			showErr(err)
			continue
		}
		et, err := runRace()
		if err != nil {
			showErr(err)
			continue
		}
		displayResults(et)
		err = waitForButtonPress()
		if err != nil {
			showErr(err)
			continue
		}
	}
}

func waitForShutdownButton() {
	for {
		bpress, err := session.DigitalRead(button2)
		if err != nil {
			continue
		}
		if bpress == 1 {
			break
		}
		time.Sleep(1 * time.Second)
	}

	lcd.SetText("SHUTDOWN")
	lcd.SetRGB(0, 255, 255)

	cmd := exec.Command("/sbin/halt")
	cmd.Run()
}

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go racebot()
	go waitForShutdownButton()

	<-sigs
	cleanup()
}
