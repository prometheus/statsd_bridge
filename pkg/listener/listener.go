package listener

import (
	"bufio"
	"io"
	"net"
	"os"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/statsd_exporter/pkg/event"
	pkgLine "github.com/prometheus/statsd_exporter/pkg/line"
)

type StatsDUDPListener struct {
	Conn         *net.UDPConn
	EventHandler event.EventHandler
	Logger       log.Logger
}

func (l *StatsDUDPListener) SetEventHandler(eh event.EventHandler) {
	l.EventHandler = eh
}

func (l *StatsDUDPListener) Listen(udpPackets prometheus.Counter, linesReceived prometheus.Counter, eventsFlushed prometheus.Counter, sampleErrors prometheus.CounterVec, samplesReceived prometheus.Counter, tagErrors prometheus.Counter, tagsReceived prometheus.Counter) {
	buf := make([]byte, 65535)
	for {
		n, _, err := l.Conn.ReadFromUDP(buf)
		if err != nil {
			// https://github.com/golang/go/issues/4373
			// ignore net: errClosing error as it will occur during shutdown
			if strings.HasSuffix(err.Error(), "use of closed network connection") {
				return
			}
			level.Error(l.Logger).Log("error", err)
			return
		}
		l.HandlePacket(buf[0:n], udpPackets, linesReceived, eventsFlushed, sampleErrors, samplesReceived, tagErrors, tagsReceived)
	}
}

func (l *StatsDUDPListener) HandlePacket(packet []byte, udpPackets prometheus.Counter, linesReceived prometheus.Counter, eventsFlushed prometheus.Counter, sampleErrors prometheus.CounterVec, samplesReceived prometheus.Counter, tagErrors prometheus.Counter, tagsReceived prometheus.Counter) {
	udpPackets.Inc()
	lines := strings.Split(string(packet), "\n")
	for _, line := range lines {
		linesReceived.Inc()
		l.EventHandler.Queue(pkgLine.LineToEvents(line, sampleErrors, samplesReceived, tagErrors, tagsReceived, l.Logger), &eventsFlushed)
	}
}

type StatsDTCPListener struct {
	Conn         *net.TCPListener
	EventHandler event.EventHandler
	Logger       log.Logger
}

func (l *StatsDTCPListener) SetEventHandler(eh event.EventHandler) {
	l.EventHandler = eh
}

func (l *StatsDTCPListener) Listen(linesReceived prometheus.Counter, eventsFlushed prometheus.Counter, tcpConnections prometheus.Counter, tcpErrors prometheus.Counter, tcpLineTooLong prometheus.Counter, sampleErrors prometheus.CounterVec, samplesReceived prometheus.Counter, tagErrors prometheus.Counter, tagsReceived prometheus.Counter) {
	for {
		c, err := l.Conn.AcceptTCP()
		if err != nil {
			// https://github.com/golang/go/issues/4373
			// ignore net: errClosing error as it will occur during shutdown
			if strings.HasSuffix(err.Error(), "use of closed network connection") {
				return
			}
			level.Error(l.Logger).Log("msg", "AcceptTCP failed", "error", err)
			os.Exit(1)
		}
		go l.HandleConn(c, linesReceived, eventsFlushed, tcpConnections, tcpErrors, tcpLineTooLong, sampleErrors, samplesReceived, tagErrors, tagsReceived)
	}
}

func (l *StatsDTCPListener) HandleConn(c *net.TCPConn, linesReceived prometheus.Counter, eventsFlushed prometheus.Counter, tcpConnections prometheus.Counter, tcpErrors prometheus.Counter, tcpLineTooLong prometheus.Counter, sampleErrors prometheus.CounterVec, samplesReceived prometheus.Counter, tagErrors prometheus.Counter, tagsReceived prometheus.Counter) {
	defer c.Close()

	tcpConnections.Inc()

	r := bufio.NewReader(c)
	for {
		line, isPrefix, err := r.ReadLine()
		if err != nil {
			if err != io.EOF {
				tcpErrors.Inc()
				level.Debug(l.Logger).Log("msg", "Read failed", "addr", c.RemoteAddr(), "error", err)
			}
			break
		}
		if isPrefix {
			tcpLineTooLong.Inc()
			level.Debug(l.Logger).Log("msg", "Read failed: line too long", "addr", c.RemoteAddr())
			break
		}
		linesReceived.Inc()
		l.EventHandler.Queue(pkgLine.LineToEvents(string(line), sampleErrors, samplesReceived, tagErrors, tagsReceived, l.Logger), &eventsFlushed)
	}
}

type StatsDUnixgramListener struct {
	Conn         *net.UnixConn
	EventHandler event.EventHandler
	Logger       log.Logger
}

func (l *StatsDUnixgramListener) SetEventHandler(eh event.EventHandler) {
	l.EventHandler = eh
}

func (l *StatsDUnixgramListener) Listen(unixgramPackets prometheus.Counter, linesReceived prometheus.Counter, eventsFlushed prometheus.Counter, sampleErrors prometheus.CounterVec, samplesReceived prometheus.Counter, tagErrors prometheus.Counter, tagsReceived prometheus.Counter) {
	buf := make([]byte, 65535)
	for {
		n, _, err := l.Conn.ReadFromUnix(buf)
		if err != nil {
			// https://github.com/golang/go/issues/4373
			// ignore net: errClosing error as it will occur during shutdown
			if strings.HasSuffix(err.Error(), "use of closed network connection") {
				return
			}
			level.Error(l.Logger).Log(err)
			os.Exit(1)
		}
		l.HandlePacket(buf[:n], unixgramPackets, linesReceived, eventsFlushed, sampleErrors, samplesReceived, tagErrors, tagsReceived)
	}
}

func (l *StatsDUnixgramListener) HandlePacket(packet []byte, unixgramPackets prometheus.Counter, linesReceived prometheus.Counter, eventsFlushed prometheus.Counter, sampleErrors prometheus.CounterVec, samplesReceived prometheus.Counter, tagErrors prometheus.Counter, tagsReceived prometheus.Counter) {
	unixgramPackets.Inc()
	lines := strings.Split(string(packet), "\n")
	for _, line := range lines {
		linesReceived.Inc()
		l.EventHandler.Queue(pkgLine.LineToEvents(line, sampleErrors, samplesReceived, tagErrors, tagsReceived, l.Logger), &eventsFlushed)
	}
}
