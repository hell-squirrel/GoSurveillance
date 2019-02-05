package main

import (
	"encoding/json"
	"flag"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"gocv.io/x/gocv"
	"image"
	"image/jpeg"
	"log"
	"os"
	"strconv"
	"time"
)

type MotionDetect struct {
	botApi            *tgbotapi.BotAPI
	botReport         bool
	deviceId          int
	chatID            int64
	onlineReport      bool
	detectionChannel  chan image.Image
	reportChannel     chan string
	reportInterval    int64
	reportContourArea float64
}

type Config struct {
	OnlineReport      bool    `json:"onlineReport"`
	Token             string  `json:"token"`
	CamDeviceId       int     `json:"camDeviceId"`
	ChatId            int64   `json:"chatId"`
	ReportInterval    int64   `json:"reportInterval"`
	ReportContourArea float64 `json:"reportContourArea"`
}

//todo refactor code
//todo configuration from bot by msgs
//todo add sys interrupt handle
func main() {

	config := getConfig()
	println(config)
	detection := Init(*config)

	if config.OnlineReport {
		log.Println("Online report enabled")
		detection.ReportToBot()
	}

	go detection.ImageStoreProcessing()

	detection.BodyDetection()
}

func getConfig() *Config {
	var config = new(Config)
	flag.BoolVar(&config.OnlineReport, "online", false, "Enables online motion detect report.")
	flag.StringVar(&config.Token, "token", "", "Bot access token.")
	flag.IntVar(&config.CamDeviceId, "camera", 0, "Camera device id in system to take data from.")
	flag.Int64Var(&config.ChatId, "chat", 0, "Chat id for reporting")
	flag.Int64Var(&config.ReportInterval, "interval", 500, "Reporting interval in milliseconds")
	flag.Float64Var(&config.ReportContourArea, "contours", 4000, "Contour area in detection processing")
	flag.Parse()
	configFile, err := os.Open("config.json")
	if err != nil {
		return config
	}
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	if err != nil {
		log.Println("Error occurred while reading config")
	}
	return config
}

func Init(config Config) *MotionDetect {
	md := new(MotionDetect)
	md.reportChannel = make(chan string)
	md.detectionChannel = make(chan image.Image)
	md.onlineReport = config.OnlineReport
	md.botReport = false
	md.chatID = config.ChatId
	md.deviceId = config.CamDeviceId
	md.reportContourArea = config.ReportContourArea
	md.reportInterval = config.ReportInterval

	if config.OnlineReport {
		if config.Token == "" {
			panic("There is no token for bot")
		}
		bot, err := tgbotapi.NewBotAPI(config.Token)
		if err != nil {
			panic("Cannot start bot reporter")
		}
		md.botApi = bot
	}
	return md
}

func (md *MotionDetect) ReportToBot() {
	go md.BotStatusChangeCheck()
	go md.HandleImageReporter()
}

func (md *MotionDetect) BotStatusChangeCheck() {

	var statusKeyboardLayout = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Enable"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Disable"),
		),
	)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, _ := md.botApi.GetUpdatesChan(u)

	for update := range updates {

		if update.Message == nil {
			continue
		}

		switch update.Message.Text {
		case "Enable":
			md.BotReportStart()
			break
		case "Disable":
			md.BotReportStop()
			break
		default:
			break
		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Motion detection:")
		msg.ReplyMarkup = statusKeyboardLayout
		_, err := md.botApi.Send(msg)
		if err != nil {
			log.Println("Error while sending message!", err.Error())
		}
	}
}

func (md *MotionDetect) ImageStoreProcessing() {
	for picture := range md.detectionChannel {
		var currentTime = time.Now()
		var dirName = strconv.Itoa(currentTime.Year()) + "_" + currentTime.Month().String() + "_" + strconv.Itoa(currentTime.Day())
		if _, err := os.Stat(dirName); os.IsNotExist(err) {
			err := os.Mkdir(dirName, os.ModePerm)
			if err != nil {
				continue
			}
		}

		imageLocation, err := os.Create(dirName + string(os.PathSeparator) + currentTime.String() + ".jpeg")
		if err != nil {
			log.Println("Failed to create TMP file " + err.Error())
			continue
		}
		err = jpeg.Encode(imageLocation, picture, nil)
		if err != nil {
			log.Println("Failed to save picture")
			continue
		}
		if md.onlineReport {
			md.reportChannel <- imageLocation.Name()
		}
	}
}

func (md *MotionDetect) HandleImageReporter() {
	for location := range md.reportChannel {
		if md.botReport {
			img := tgbotapi.NewPhotoUpload(md.chatID, location)
			_, err := md.botApi.Send(img)
			if err != nil {
				log.Println("Cannot send " + err.Error())
			}
			//todo decide what to do with old files (add some time invalidation)
			//os.Remove(location)
		}
	}
}

func (md *MotionDetect) BotReportStart() {
	if !md.onlineReport {
		return
	}
	md.botReport = true
}

func (md *MotionDetect) BotReportStop() {
	if !md.onlineReport {
		return
	}
	md.botReport = false
}

func (md *MotionDetect) BodyDetection() {

	log.Println("Body detection started...")

	deviceID := md.deviceId

	webcam, err := gocv.OpenVideoCapture(deviceID)
	if err != nil {
		log.Println("Error opening video capture device:", deviceID, err.Error())
		return
	}
	defer webcam.Close()

	img := gocv.NewMat()
	defer img.Close()

	imgDelta := gocv.NewMat()
	defer imgDelta.Close()

	imgThresh := gocv.NewMat()
	defer imgThresh.Close()

	mog2 := gocv.NewBackgroundSubtractorMOG2()
	defer mog2.Close()

	log.Println("with reading from device:", deviceID)

	var expire = time.Now().Add(time.Duration(md.reportInterval) * time.Millisecond).UnixNano()
	for {
		if ok := webcam.Read(&img); !ok {
			log.Println("Device closed:", deviceID)
			return
		}
		if img.Empty() {
			continue
		}

		mog2.Apply(img, &imgDelta)
		gocv.Threshold(imgDelta, &imgThresh, 25, 255, gocv.ThresholdBinary)
		kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(3, 3))
		gocv.Dilate(imgThresh, &imgThresh, kernel)
		contours := gocv.FindContours(imgThresh, gocv.RetrievalExternal, gocv.ChainApproxSimple)
		//defer kernel.Close() DONT USE IN LOOP =)
		for _, c := range contours {
			area := gocv.ContourArea(c)
			if area < md.reportContourArea {
				kernel.Close()
				continue
			}
			if expire < time.Now().UnixNano() {
				imgWithMovement, err := img.ToImage()
				if err != nil {
					kernel.Close()
					continue
				}
				md.detectionChannel <- imgWithMovement
				expire = time.Now().Add(500 * time.Millisecond).UnixNano()
			}
		}
		kernel.Close()
	}
}
