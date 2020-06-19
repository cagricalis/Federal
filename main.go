package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"mime/quotedprintable"
	"net"
	"net/http"
	"net/smtp"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var exUnlockCommand = "[QUNLOCK:2]"
var resetCommand = "[QRESET:OK123!]"
var unlockCommand = "[QUNLOCK:<lock_id>]"
var lockCommand = "[QLOCK:<lock_id>]"
var sendSMSCommand = "[QSMS:<phoneno>:<smsbody>:OK123!]"
var cimiCommand = "[OHUB:CIMI:<cimistr>:OK]"
var lockStatusCommand = "[STATUS:<nbrpar>:iiiiiiiiiiii]"

func sendUnlockCommand() {
	writeHub([]byte(exUnlockCommand))
}

func sendEmail(tcKimlik string, gasType string, lockNumber string) {
	fromemail := "aygaztupmatik@gmail.com"
	password := "f)S3eg-n"
	host := "smtp.gmail.com:587"
	auth := smtp.PlainAuth("", fromemail, password, "smtp.gmail.com")

	header := make(map[string]string)
	toemail := "cagricalis@gmail.com"
	header["From"] = fromemail
	header["To"] = toemail
	header["Subject"] = "AYGAZ Tupmatik Yeni Satis Gerceklesti"

	header["MIME-Version"] = "1.0"
	header["Content-Type"] = fmt.Sprintf("%s; charset=\"utf-8\"", "text/html")
	header["Content-Transfer-Encoding"] = "quoted-printable"
	header["Content-Disposition"] = "inline"

	headermessage := ""
	for key, value := range header {
		headermessage += fmt.Sprintf("%s: %s\r\n", key, value)
	}
	var newMessageFromDevice = fmt.Sprintf("TC KIMLIK: %s, Satis Yapilan Locker Numarası: %s, Satılan Urun Tipi: %s \r\n", tcKimlik, lockNumber, gasType)

	body := "<h1>" + newMessageFromDevice + "</h1>"
	var bodymessage bytes.Buffer
	temp := quotedprintable.NewWriter(&bodymessage)
	temp.Write([]byte(body))
	temp.Close()

	finalmessage := headermessage + "\r\n" + bodymessage.String()
	status := smtp.SendMail(host, auth, fromemail, []string{toemail}, []byte(finalmessage))
	if status != nil {
		log.Printf("Error from SMTP Server: %s", status)
	}
	log.Print("Email Sent Successfully")
}

var aygazSale = "AYGAZSALE"
var aygazInfo = "AYGAZINFO"
var aygazOpenLockInfo = "AYGAZOPENLOCKINFO"
var hub *net.Conn
var clients []*websocket.Conn

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var hubStatus = "[QR:LOCKS:1111100111999999:OK]"

func handleWebsocket() {
	log.Println("handleWebsocket()")
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)

		if err != nil {
			log.Println(err.Error())
			deleteClient(conn)
			return
		}

		log.Println("Client connected!")
		clients = append(clients, conn)

		if err = conn.WriteMessage(1, []byte(hubStatus)); err != nil {
			log.Println(err.Error())
			log.Println("CLIENT WRITE MESSAGE ERROR")
		}

		for {
			_, data, err := conn.ReadMessage()

			if err != nil {
				log.Println(err.Error())
				log.Println("read message error")
				continue
			}

			log.Println("Client:", string(data))
			writeHub(data)

			time.AfterFunc(3*time.Second, sendHubStatus)

			writeClients(string(data))
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "client/index.html")
	})
}

func acceptTCP(sw net.Listener) {
	log.Println("acceptTCP()")
	for {
		connection, err := sw.Accept()

		if err != nil {
			log.Println(err.Error())
			continue
		}

		log.Println("Accepted hub connection!")

		go handleTCP(&connection)
	}
}

func sendHubStatus() {
	writeHub([]byte("[QSTATUS?]"))
}
func sendUnlock() {
	writeHub([]byte("[QUNLOCK:1]"))
}
func sendDebugCommand() {
	writeHub([]byte("[DEBUG]"))
}

var headData = "headData"
var tcKimlik = "1234567890"
var gasType = "gri"
var lockNumber = "5"

func handleTCP(c *net.Conn) {
	log.Println("HandleTCP()")
	log.Println("Hub connection request:", (*c).RemoteAddr().String())
	hub = c

	for {
		netData, err := bufio.NewReader(*c).ReadString('\n')
		netData = strings.TrimSpace(string(netData))
		split1 := strings.Split(netData, ":")
		splitted := split1[0]
		splitted2 := split1[3]

		headData = strings.Trim(splitted, "[")

		if aygazSale == headData {
			log.Println("AYGAZSALE READED")

			log.Printf("%s Nolu dolaptan satis", split1[1])
			if split1[2] == "G" {
				log.Println("GRI SATILDI")
				gasType = "Gri"
			} else if split1[2] == "Y" {
				log.Println("YESIL SATILDI")
				gasType = "Yesil"
			}
			tcKimlik = strings.Trim(splitted2, "]")
			log.Printf("TC KIMLIK NO: %s\n", tcKimlik)
			sendEmail(tcKimlik, gasType, lockNumber)

		} else if aygazInfo == headData {
			log.Println("AYGAZINFO READED")
			log.Printf("Tup Tipleri: %s", split1[1])
		} else if aygazOpenLockInfo == headData {
			log.Println("AYGAZOPENLOCKINFO READED")
		}

		if err != nil {
			log.Println("Hub disconnected:", err)
			hub = nil
			break
		}
		log.Println(string(netData))

		if matches, err := regexp.Match(`^\[QR:LOCKS:\d{16}:OK]$`, []byte(netData)); err != nil {
			log.Println(err.Error())
			log.Println("NOT MATCHES!")
		} else if matches {
			log.Println("New hub status received:", netData)
			hubStatus = netData
		} else {
			log.Println("Hub:", netData)
		}

		writeClients(netData)
	}

	if err := (*c).Close(); err != nil {
		log.Println(err.Error())
	}
}

func writeHub(data []byte) {
	if hub != nil {
		if _, err := (*hub).Write(data); err != nil {
			log.Println(err.Error())
		}
	}
}

func writeClients(data string) {
	for i := 0; i < len(clients); i++ {
		if err := clients[i].WriteMessage(1, []byte(string(data))); err != nil {
			log.Println(err.Error())
		}
	}
}

func main() {

	log.Println("Started listening TCP on port: 9009")
	log.Println("Started listening Websocket on port: 9010")

	// Listen TCP Clients
	sw, err := net.Listen("tcp4", ":9008")

	if err != nil {
		log.Println(err.Error())
		return
	}

	go acceptTCP(sw)

	// Start Status Ticker
	ticker := time.NewTicker(30000 * time.Millisecond)
	go func() {
		for range ticker.C {
			sendHubStatus()
			sendDebugCommand()
		}
	}()

	// Listen Websocket Clients
	handleWebsocket()

	if err := http.ListenAndServe(":9009", nil); err != nil {
		log.Println(err.Error())
		return
	}
}

func deleteClient(c *websocket.Conn) {
	for i := 0; i < len(clients); i++ {
		if clients[i] == c {
			clients[i] = clients[len(clients)-1]
			clients[len(clients)-1] = nil
			clients = clients[:len(clients)-1]
			return
		}
	}
}
