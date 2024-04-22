package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	// "sync"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"nesty.cn/bee"
)

func ReadWebSocket(c *websocket.Conn, buff []byte) (int, int, []byte, error) {

	buff = buff[:0]

	var r io.Reader
	messageType, r, err := c.NextReader()
	if err != nil {
		return messageType, 0, buff, err
	}
	nreadloop := 1

	n, err := r.Read(buff)

	if err == io.EOF {
		err = nil
		//fmt.Println("0 totread:",n,"readloop:",nreadloop)
		return messageType, n, buff, err
	}

	nmin := 1024
	nmax := 0
	// n0 := n

	ntot := n
	b := make([]byte, 10240)
	// istart := n
	for {
		nreadloop++
		n, err = r.Read(b)
		if n > 0 {
			//istart, buff = Memcpy(buff, istart, b, n)
			// bee.Log("buff.cap1", cap(buff))
			buff = append(buff, b[:n]...)
			// bee.Log("buff.cap2:", cap(buff))
			ntot += n

			if n < nmin {
				nmin = n
			} else if n > nmax {
				nmax = n
			}
		}

		if err == io.EOF {
			err = nil
			// fmt.Println("nreadloop:", nreadloop, "min:", nmin, "max:", nmax, (ntot-n0)/(nreadloop-1))
			// if ntot > 1000 {
			// 	readPacketMin = nmin
			// 	readPacketMax = nmax
			// 	readPacketLoops = nreadloop
			// 	readPacketAvg = (ntot - n0) / (nreadloop - 1)
			// }
			return messageType, ntot, buff, err
		} else if err != nil {
			break
		}
		// time.Sleep(100 * time.Nanosecond)
	}
	//fmt.Println("2 totread:",ntot,"readloop:",nreadloop)
	return messageType, ntot, buff, err
}

func main() {
	fmt.Println("------------main start----------")

	var addr string
	var nsend int
	var nloop int
	var nping int
	var nblockdownload int
	var scompress string
	var web string
	flag.StringVar(&addr, "h", "", "")
	flag.IntVar(&nsend, "n", 0, "number of block(10M) for each loop to send")
	flag.IntVar(&nloop, "l", 0, "number of loop to send")
	flag.IntVar(&nping, "p", 10, "number of ping test")
	flag.IntVar(&nblockdownload, "d", 0, "number of block(10M) to download")
	flag.StringVar(&scompress, "c", "false", "enable compress")
	flag.StringVar(&web, "w", "auto", "internet")
	flag.Parse()

	bCompress := bee.Str2Bool(scompress)

	bee.Log("enable compress:", bCompress)

	if len(addr) < 1 && len(flag.Args()) > 0 {
		addr = flag.Args()[0]
	}

	if len(addr) < 1 {
		fmt.Println("target host cannot be empty")
		return
	}

	if !strings.Contains(addr, ":") {
		addr += ":8080"
	}

	var t0 int64 = 0
	var t1 int64 = 0

	// make(chan os.Signal, 1)

	u := url.URL{Scheme: "ws", Host: addr, Path: "/ws"}
	fmt.Printf("connecting to %s\n", u.String())

	// c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	dial := websocket.DefaultDialer
	dial.EnableCompression = bCompress
	c, _, err := dial.Dial(u.String(), nil)
	if err != nil {
		fmt.Println("dial:", err)
		return
	}
	defer c.Close()

	done := make(chan struct{})

	tping := int64(0)
	iping := 0
	var sendDone bool = false
	var isendloop int = 0
	var ntot int = 0
	var ntotdownload int64 = 0
	var aspeed float64 = 0
	// var aspeedr float64 = 0
	var idownload int = 0
	var totalTimeSend int64 = 0

	chanJobDone := make(chan bool, 1)

	var tpingMin int64 = 0
	var tpingMax int64 = 0
	var atping float64 = 0

	go func() {
		defer close(done)
		readBuf := make([]byte, 1024*1024*10+1)
		for {
			// _, p, err := c.ReadMessage()
			var nread int = 0
			//readBuf = readBuf[:0]
			_, nread, readBuf, err = ReadWebSocket(c, readBuf) //new code
			if err != nil {
				fmt.Println("read:", err)
				return
			}
			// nread = len(p)
			if nread < 1 {
				continue
			}
			p := readBuf
			if p[0] == 2 {
				//b := p[1:]
				//smsg := string(b)
				sendDone = true
				isendloop++
				// fmt.Println("server told:", smsg)
				t1 := time.Now().UnixNano() / 1000000
				dt := int64(t1 - t0)
				speed := float64(ntot) / 1024.0 / 1024 * 1000 / float64(dt)
				t0 = t1
				totalTimeSend += dt
				fmt.Printf("total send [%2d/%v] %v MB, speed: %8.3f MB/S\n", isendloop, nloop, ntot/1024.0/1024, speed)
				aspeed += speed
				if isendloop == nloop {
					fmt.Printf("average upload speed: %.3f MB/S\n", aspeed/float64(nloop))
					fmt.Printf("total time elasped: %.2f S\n\n", float64(totalTimeSend)/1000)
				}
			} else if p[0] == 3 {
				t1 = time.Now().UnixNano() / 1000
				iping++
				dt := t1 - t0
				t0 = t1
				tping += int64(dt)
				fmt.Printf("ping test [%2d/%v] time %8.3f ms\n", iping, nping, float64(dt)/1000.0)
				if tpingMin < 1 {
					tpingMin = tping
				}
				if tping < dt {
					tpingMin = dt
				}
				if tping > dt {
					tpingMax = dt
				}
				if iping == nping {
					atping = float64(tping) * 1.0 / float64(nping) / 1000.0
					fmt.Printf("average ping time: %.3f ms, # test: %v\n\n", atping, nping)
				}
			} else if p[0] == 0 {
				idownload++
				t1 := time.Now().UnixNano() / 1e3
				dt := t1 - t0
				// t0 = t1
				ntotdownload += int64(nread)
				speed := float64(ntotdownload) / 1024.0 / 1024.0 * 1000000 / float64(dt)
				fmt.Printf("download [%2d/%v] %v MB, speed: %8.3f MB/S\n", idownload, nblockdownload, nread/1024/1024, speed)
				if ntotdownload == int64(nblockdownload)*10*1024*1024 {
					chanJobDone <- true
					break
				}
			}
			//fmt.Printf("recv: %s", message)
		}
	}()

	//ping tests
	for i := 0; i < nping; i++ {
		bd := []byte{3}
		t0 = time.Now().UnixNano() / 1e3
		t1 = 0
		c.WriteMessage(websocket.BinaryMessage, bd)
		time.Sleep(time.Millisecond * 100)
		for t1 == 0 {
			time.Sleep(time.Millisecond * 100)
		}
	}

	fmt.Println("min:", tpingMin, "max:", tpingMax, atping)
	if web == "auto" {
		if tpingMin < 800 || atping < 2 {
			web = ""
			fmt.Println("------------local net test------------")
		} else {
			web = "web"
			fmt.Println("---------- intertnet test ------------")
		}
	}

	if web == "" {
		if nsend < 1 {
			nsend = 10
		}
		if nloop < 1 {
			nloop = 10
		}
		if nblockdownload < 1 {
			nblockdownload = 50
		}
	} else {
		if nsend < 1 {
			nsend = 1
		}
		if nloop < 1 {
			nloop = 1
		}
		if nblockdownload < 1 {
			nblockdownload = 1
		}
	}

	//upload tests
	bd := make([]byte, 1024*1024*10)
	t0 = time.Now().UnixNano() / 1000000
	c.WriteMessage(websocket.BinaryMessage, []byte{1})

	isendloop = 0
	for l := 0; l < nloop; l++ {
		sendDone = false
		ntot = 0
		for i := 0; i < nsend; i++ {
			c.WriteMessage(websocket.BinaryMessage, bd)
			//fmt.Printf("send %v\n", i+1)
			ntot += len(bd)
		}
		c.WriteMessage(websocket.BinaryMessage, []byte{2})
		for !sendDone {
			time.Sleep(1 * time.Millisecond)
		}
	}

	//download tests
	t0 = time.Now().UnixNano() / 1e3
	ntotdownload = 0
	c.WriteMessage(websocket.BinaryMessage, []byte{4, byte(nblockdownload / 256), byte(nblockdownload % 256)})
	<-chanJobDone

	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye bye")
	c.WriteMessage(websocket.CloseMessage, closeMsg)
	time.Sleep(100)

}
