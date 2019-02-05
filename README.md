# GoSurveillance
Application for surveillance based on movement detection via webcam. Supported reporting to TelegramBot.
### Required:
>OpenCV v4

[Go OpenCV install instruction](gocv.io/x/gocv)
---
Surveillance result aka 'Motion capture' by default is stored in folders within timestamps.
With Telegram Bot token and chatId application can send realtime detection results.

#### Configuration

---
Configuration is present in both options, app args and config file.

###### Arguments usage:
- camera int
  - Camera device id in system to take data from
- contours float
  - [Contour area in detection processing (default 4000)]((https://docs.opencv.org/3.3.0/d3/dc0/group__imgproc__shape.html#ga2c759ed9f497d4a618048a2f56dc97f1))
- interval int
  - Reporting interval in milliseconds (default 500 milliseconds)
- online bool
  - Enables online motion detect report
- chat int
  - Chat id for reporting
- token string
  - Bot access token
###### Config file example
```
{
   "onlineReport": false,
   "token": "787934588:BBHfTwgT78NmQUSFRfyjZeT2-erttEiTKvF",
   "camDeviceId": 2,
   "chatId": 123455215,
   "reportInterval": 1500,
   "reportContourArea": 3200
}
```
