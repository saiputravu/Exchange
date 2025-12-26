package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"fenrir/internal/common"
	fenrirNet "fenrir/internal/net"
)

// reportFixedHeaderLen matches the server's expectation:
// 1+1+1+8+8+8+2+4+4+16 = 53 bytes.
const reportFixedHeaderLen = 53

func main() {
	// 1. CLI Parameter Parsing
	serverAddr := flag.String("server", "127.0.0.1:9001", "Address of the exchange server")
	owner := flag.String("owner", "", "Owner username (compulsory)")
	action := flag.String("action", "place", "Action to perform: ['place', 'cancel', 'log']")

	// Order Parameters
	ticker := flag.String("ticker", "AAPL", "Ticker symbol (max 4 chars)")
	sideStr := flag.String("side", "buy", "Order side: 'buy' or 'sell'")
	typeStr := flag.String("type", "limit", "Order type: 'limit' or 'market'")
	price := flag.Float64("price", 100.0, "Limit price")
	qtyStr := flag.String("qty", "10", "Quantity or comma-separated list (e.g. 10,20,50)")

	// Cancel Parameters
	uuid := flag.String("uuid", "", "UUID of the order to cancel")

	flag.Parse()

	// Validation
	if *owner == "" {
		fmt.Println("Error: -owner is compulsory.")
		flag.Usage()
		os.Exit(1)
	}

	// Connect to Server
	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Failed to connect to server at %s: %v", *serverAddr, err)
	}
	defer conn.Close()
	fmt.Printf("Connected to %s as '%s'\n", *serverAddr, *owner)

	// Start Listening for Reports (Async)
	go readReports(conn)

	// Prepare Enums using 'common' package
	side := common.Buy
	if strings.ToLower(*sideStr) == "sell" {
		side = common.Sell
	}

	orderType := common.LimitOrder
	if strings.ToLower(*typeStr) == "market" {
		orderType = common.MarketOrder
	}

	// Execute Action
	switch strings.ToLower(*action) {
	case "place":
		quantities := parseQuantities(*qtyStr)
		for _, q := range quantities {
			// Using common.Equities as the default AssetType
			err := sendPlaceOrder(conn, *owner, common.Equities, orderType, *ticker, *price, q, side)
			if err != nil {
				log.Printf("Failed to place order (Qty: %d): %v", q, err)
			} else {
				fmt.Printf("-> Sent %s Order: %s %d @ %.2f\n", strings.ToUpper(*sideStr), *ticker, q, *price)
			}
			// Small optional sleep to ensure server processes sequence distinctly if needed
			time.Sleep(5 * time.Millisecond)
		}

	case "cancel":
		if *uuid == "" {
			log.Fatal("Error: -uuid is required for cancellation")
		}
		// Using common.Equities for cancel as well
		err := sendCancelOrder(conn, common.Equities, *uuid)
		if err != nil {
			log.Printf("Failed to send cancel request: %v", err)
		} else {
			fmt.Printf("-> Sent Cancel Request for UUID: %s\n", *uuid)
		}

	case "log":
		err := sendLog(conn)
		if err != nil {
			log.Printf("Failed to send log request: %v", err)
		} else {
			fmt.Println("-> Sent Log Request")
		}

	default:
		log.Fatalf("Unknown action: %s", *action)
	}

	// Keep the client alive to receive execution reports
	fmt.Println("\nListening for reports... (Press Ctrl+C to exit)")
	select {}
}

// parseQuantities splits a comma-separated string into a slice of uint64
func parseQuantities(input string) []uint64 {
	parts := strings.Split(input, ",")
	var result []uint64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if val, err := strconv.ParseUint(p, 10, 64); err == nil {
			result = append(result, val)
		} else {
			log.Printf("Warning: Invalid quantity '%s', skipping.", p)
		}
	}
	return result
}

// sendPlaceOrder constructs and sends the NewOrder message
func sendPlaceOrder(conn net.Conn, owner string, asset common.AssetType, orderType common.OrderType, ticker string, price float64, qty uint64, side common.Side) error {
	usernameLen := len(owner)

	// We must include BaseMessageHeaderLen (2) in the total size.
	// Previous calculation was: NewOrderMessageHeaderLen (26) + usernameLen.
	// This was 2 bytes short, causing truncation of the username.
	totalLen := fenrirNet.BaseMessageHeaderLen + fenrirNet.NewOrderMessageHeaderLen + usernameLen

	buf := make([]byte, totalLen)

	// 1. Header (TypeOf = NewOrder)
	binary.BigEndian.PutUint16(buf[0:2], uint16(fenrirNet.NewOrder))

	// 2. Body
	// internal/net/messages.go expects AssetType and OrderType as uint16
	binary.BigEndian.PutUint16(buf[2:4], uint16(asset))
	binary.BigEndian.PutUint16(buf[4:6], uint16(orderType))

	// Ticker (Pad or truncate to 4 bytes)
	tickerBytes := make([]byte, 4)
	copy(tickerBytes, ticker)
	copy(buf[6:10], tickerBytes)

	binary.BigEndian.PutUint64(buf[10:18], math.Float64bits(price))
	binary.BigEndian.PutUint64(buf[18:26], qty)

	// Side is cast to byte/uint8
	buf[26] = byte(side)
	buf[27] = uint8(usernameLen)

	// Copy owner name into buffer
	// buf[28:] now has sufficient space for the full username
	copy(buf[28:], owner)

	_, err := conn.Write(buf)
	return err
}

// sendCancelOrder constructs and sends the CancelOrder message
func sendCancelOrder(conn net.Conn, asset common.AssetType, uuid string) error {
	// Using exported constants from fenrir/internal/net
	buf := make([]byte, fenrirNet.CancelOrderMessageHeaderLen)

	// 1. Header (TypeOf = CancelOrder)
	binary.BigEndian.PutUint16(buf[0:2], uint16(fenrirNet.CancelOrder))

	// 2. Body
	binary.BigEndian.PutUint16(buf[2:4], uint16(asset))

	// UUID (Truncate or pad to 16 bytes)
	uuidBytes := make([]byte, 16)
	copy(uuidBytes, uuid)
	copy(buf[4:20], uuidBytes)

	_, err := conn.Write(buf)
	return err
}

func sendLog(conn net.Conn) error {
	buf := make([]byte, fenrirNet.BaseMessageHeaderLen)
	binary.BigEndian.PutUint16(buf[0:2], uint16(fenrirNet.LogBook))
	_, err := conn.Write(buf)
	return err
}

// readReports continuously reads and parses Report messages from the server
func readReports(conn net.Conn) {
	for {
		// 1. Read Fixed Header
		headerBuf := make([]byte, reportFixedHeaderLen)
		_, err := io.ReadFull(conn, headerBuf)
		if err != nil {
			if err != io.EOF {
				log.Printf("Connection lost: %v", err)
			}
			os.Exit(0)
		}

		// 2. Parse Fixed Fields
		msgType := fenrirNet.ReportMessageType(headerBuf[0])
		side := common.Side(headerBuf[2])

		qty := binary.BigEndian.Uint64(headerBuf[11:19])
		price := math.Float64frombits(binary.BigEndian.Uint64(headerBuf[19:27]))
		counterpartyLen := binary.BigEndian.Uint16(headerBuf[27:29])
		errStrLen := binary.BigEndian.Uint32(headerBuf[29:33])

		ticker := string(headerBuf[33:37])
		uuid := string(headerBuf[37:53])

		// 3. Read Variable Length Strings (Error and Counterparty)
		totalVarLen := int(counterpartyLen) + int(errStrLen)
		varBuf := make([]byte, totalVarLen)
		if totalVarLen > 0 {
			_, err := io.ReadFull(conn, varBuf)
			if err != nil {
				log.Printf("Error reading report body: %v", err)
				break
			}
		}

		// Extract Strings
		errStr := ""
		counterparty := ""
		if errStrLen > 0 {
			errStr = string(varBuf[:errStrLen])
		}
		if counterpartyLen > 0 {
			counterparty = string(varBuf[errStrLen:])
		}

		// 4. Print Report using imported Enums
		if msgType == fenrirNet.ErrorReport {
			fmt.Printf("\n[SERVER ERROR] %s\n", errStr)
		} else {
			sideStr := "BUY"
			if side == common.Sell {
				sideStr = "SELL"
			}
			fmt.Printf("\n[EXECUTION] Match: %s %s | Qty: %d | Price: %.2f | vs: %s | UUID: %s\n",
				sideStr, ticker, qty, price, counterparty, strings.TrimRight(uuid, "\x00"))
		}
	}
}
