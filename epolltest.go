package main

import (
	"fmt"
	"log"
	"net"
	"syscall"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/sys/unix"
)

// const(
// 	EPOLLET 1 << 31
// 	MaxEpollEvents = 1
// )

type epoll struct {
	fd int
}

type timerfd struct {
	fd int
}

const (
	EPOLLET        = 1 << 31
	MaxEpollEvents = 30
)

func initEpoll() (epoll, error) {
	epfd, err := unix.EpollCreate1(0)
	if err != nil {
		return epoll{}, err
	}
	return epoll{fd: epfd}, nil
}

func (ep *epoll) close() {
	unix.Close(ep.fd)
}

func (ep *epoll) wait() ([]unix.EpollEvent, error) {
	var events [MaxEpollEvents]unix.EpollEvent
	nevents, err := unix.EpollWait(ep.fd, events[:], -1)
	//fmt.Println("Returned events:", nevents)
	if err != nil {
		return []unix.EpollEvent{}, err
	}

	return events[:nevents], nil

}

func (ep *epoll) add(fd int, eventOperations uint32, edgeMode bool) error {
	// fmt.Println("epoll add:", fd)
	var event unix.EpollEvent
	event.Events = eventOperations
	if edgeMode {
		event.Events |= unix.EPOLLET
	}
	event.Fd = int32(fd)
	if err := unix.EpollCtl(ep.fd, unix.EPOLL_CTL_ADD, fd, &event); err != nil {
		return err
	}
	return nil
}

func sendICMP(conn *net.IPConn, id int, sequence int, ipaddr net.IP) error {

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  sequence,
			Data: []byte(""),
		},
	}
	b, err := msg.Marshal(nil)
	if err != nil {
		return err
	}

	if _, err := conn.WriteTo(b, &net.IPAddr{IP: ipaddr, Zone: ""}); err != nil {
		if networkErr, ok := err.(*net.OpError); ok {
			if networkErr.Err == syscall.ENOBUFS {
				return nil
			}
		}
	}
	return err
}

func main() {

	// var tfd int
	// var ep epoll
	// var err error

	ep, err := initEpoll()
	if err != nil {
		panic(err)
	}

	defer ep.close()

	//Creating the fd for icmp reception and registration to the epoll instance

	// daddr, err := net.ResolveIPAddr("ip4:icmp", "192.168.9.33")
	// if err != nil {
	// 	panic(err)
	// }

	conn, err := net.ListenIP("ip4:icmp", &net.IPAddr{IP: net.ParseIP("192.168.9.34")}) //receive connection. Local IP address is specified at the second arg
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	rcvf, err := conn.File() //receive file
	if err != nil {
		panic(err)
	}

	rcvfd := rcvf.Fd() //receive fd
	err = ep.add(int(rcvfd), unix.EPOLLIN, true)
	if err != nil {
		panic(err)
	}

	// Setting the timerfd and registration to the epoll instance
	tfd, err := unix.TimerfdCreate(unix.CLOCK_MONOTONIC, unix.TFD_NONBLOCK|unix.TFD_CLOEXEC)
	if err != nil {
		panic(err)
	}

	if err := ep.add(tfd, unix.EPOLLIN, true); err != nil {
		panic(err)
	}

	newValue := &unix.ItimerSpec{
		Value: unix.Timespec{
			Sec:  1,
			Nsec: 0,
		},
		Interval: unix.Timespec{
			Sec:  1,
			Nsec: 0,
		},
	}

	if err := unix.TimerfdSettime(tfd, 0, newValue, nil); err != nil {
		panic(err)
	}
	//

	// Getting ready for transmissio of icmp packets
	//

	i := 0
	fmt.Println("Begining the waiting loop")
	var events []unix.EpollEvent
	for {
		if events, err = ep.wait(); err != nil {

			fmt.Println(err)
			if err.Error() != "interrupted system call" {
				return
			}
		}
		log.Println("# of events: ", len(events))
		for _, event := range events {
			// var val uint64
			buf := make([]byte, 65536)
			if int(event.Fd) == tfd {
				i++
				//_, _ = syscall.Read(int(event.Fd), (*(*[8]byte)(unsafe.Pointer(&val)))[:])
				// var val uint64
				// _, _ = unix.Read(int(event.Fd), (*(*[8]byte)(unsafe.Pointer(&val)))[:])
				_, _ = unix.Read(tfd, buf)
				log.Println("transmit an icmp: i: ", i)
				sendICMP(conn, 2, 1, net.ParseIP("172.24.1.2"))
				sendICMP(conn, 2, 1, net.ParseIP("172.23.3.6")) //endpoint address is specified at the 4'th argument
				log.Println("TX done")

			} else if int(event.Fd) == int(rcvfd) {
				//Consumption of the packet process is demanded. Otherwise, the kernel space would become full of un-consumed packets
				//if we want to use the level-triggered mode
				//_, _ = unix.Read(int(event.Fd), (*(*[8]byte)(unsafe.Pointer(&val)))[:])
				// buf := make([]byte, 65536)
				n, addr, err := conn.ReadFrom(buf)
				if err != nil {
					continue
				}
				rm, _ := icmp.ParseMessage(1, buf[:n])
				switch rm.Type {

				case ipv4.ICMPTypeEchoReply:
					log.Println("icmp replied by: ", addr.String())
				}
			}
		}

		if i > 100 {
			break
		}
	}
	return
}
