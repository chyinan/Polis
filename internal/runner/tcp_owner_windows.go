//go:build windows

// pattern: Imperative Shell
package runner

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	ipHelperIPv4                    = 2
	ipHelperIPv6                    = 23
	tcpTableOwnerPIDListener        = 3
	tcpTableOwnerPIDAll             = 5
	tcpStateListen                  = 2
	tcpStateEstablished             = 5
	errorInsufficientBuffer  uint32 = 122
	maxWindowsTCPTableBytes         = 16 << 20
)

var (
	ErrTCPListenerOwnerUnverified = errors.New("Windows loopback TCP listener owner is unverified")
	iphlpapiDLL                   = windows.NewLazySystemDLL("iphlpapi.dll")
	getExtendedTCPTableProc       = iphlpapiDLL.NewProc("GetExtendedTcpTable")
)

type mibTCPRowOwnerPID struct {
	state      uint32
	localAddr  uint32
	localPort  uint32
	remoteAddr uint32
	remotePort uint32
	processID  uint32
}

type mibTCPTableOwnerPID struct {
	count uint32
	rows  [1]mibTCPRowOwnerPID
}

type mibTCP6RowOwnerPID struct {
	localAddr     [16]byte
	localScopeID  uint32
	localPort     uint32
	remoteAddr    [16]byte
	remoteScopeID uint32
	remotePort    uint32
	state         uint32
	processID     uint32
}

type mibTCP6TableOwnerPID struct {
	count uint32
	rows  [1]mibTCP6RowOwnerPID
}

// VerifyWindowsTCPListenerOwner confirms that the exact loopback listener is
// currently owned by processID using the Windows IP Helper owner-PID tables.
func VerifyWindowsTCPListenerOwner(ctx context.Context, processID int, bindAddress string, port uint16) error {
	if ctx == nil || processID <= 0 || port == 0 {
		return ErrTCPListenerOwnerUnverified
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var err error
	switch bindAddress {
	case "127.0.0.1":
		err = verifyWindowsIPv4TCPListenerOwner(ctx, processID, port)
	case "::1":
		err = verifyWindowsIPv6TCPListenerOwner(ctx, processID, port)
	default:
		return ErrTCPListenerOwnerUnverified
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return err
}

func verifyWindowsIPv4TCPListenerOwner(ctx context.Context, processID int, port uint16) error {
	ipv4Table, err := queryWindowsTCPTable(ctx, ipHelperIPv4, tcpTableOwnerPIDListener)
	if err != nil {
		return err
	}
	rowOffset := int(unsafe.Offsetof(mibTCPTableOwnerPID{}.rows))
	rowSize := int(unsafe.Sizeof(mibTCPRowOwnerPID{}))
	count, ok := windowsTCPTableCount(ipv4Table, rowOffset, rowSize)
	if !ok {
		return errors.New("Windows IPv4 TCP listener table is malformed")
	}
	rows := make([]tcpListenerOwner, 0, count)
	for index := uint32(0); index < count; index++ {
		row := ipv4Table[rowOffset+int(index)*rowSize:]
		if binary.LittleEndian.Uint32(row[0:4]) != tcpStateListen {
			continue
		}
		rows = append(rows, tcpListenerOwner{
			family: tcpListenerIPv4, address: net.IP(append([]byte(nil), row[4:8]...)),
			port: binary.BigEndian.Uint16(row[8:10]), processID: int(binary.LittleEndian.Uint32(row[20:24])),
		})
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	ipv6Table, err := queryWindowsTCPTable(ctx, ipHelperIPv6, tcpTableOwnerPIDListener)
	if err != nil {
		return err
	}
	ipv6Offset := int(unsafe.Offsetof(mibTCP6TableOwnerPID{}.rows))
	ipv6Size := int(unsafe.Sizeof(mibTCP6RowOwnerPID{}))
	ipv6Count, ok := windowsTCPTableCount(ipv6Table, ipv6Offset, ipv6Size)
	if !ok {
		return errors.New("Windows IPv6 TCP listener table is malformed")
	}
	for index := uint32(0); index < ipv6Count; index++ {
		row := ipv6Table[ipv6Offset+int(index)*ipv6Size:]
		if binary.LittleEndian.Uint32(row[48:52]) != tcpStateListen {
			continue
		}
		rows = append(rows, tcpListenerOwner{
			family: tcpListenerIPv6, address: net.IP(append([]byte(nil), row[0:16]...)),
			port: binary.BigEndian.Uint16(row[20:22]), processID: int(binary.LittleEndian.Uint32(row[52:56])),
		})
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if !tcpListenerOwnerIsUnambiguous(rows, processID, "127.0.0.1", port) {
		return ErrTCPListenerOwnerUnverified
	}
	return nil
}

func verifyWindowsIPv6TCPListenerOwner(ctx context.Context, processID int, port uint16) error {
	table, err := queryWindowsTCPTable(ctx, ipHelperIPv6, tcpTableOwnerPIDListener)
	if err != nil {
		return err
	}
	rowOffset := int(unsafe.Offsetof(mibTCP6TableOwnerPID{}.rows))
	rowSize := int(unsafe.Sizeof(mibTCP6RowOwnerPID{}))
	count, ok := windowsTCPTableCount(table, rowOffset, rowSize)
	if !ok {
		return errors.New("Windows IPv6 TCP listener table is malformed")
	}
	rows := make([]tcpListenerOwner, 0, count)
	for index := uint32(0); index < count; index++ {
		row := table[rowOffset+int(index)*rowSize:]
		if binary.LittleEndian.Uint32(row[48:52]) != tcpStateListen {
			continue
		}
		rows = append(rows, tcpListenerOwner{
			family: tcpListenerIPv6, address: net.IP(append([]byte(nil), row[0:16]...)),
			port: binary.BigEndian.Uint16(row[20:22]), processID: int(binary.LittleEndian.Uint32(row[52:56])),
		})
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if !tcpListenerOwnerIsUnambiguous(rows, processID, "::1", port) {
		return ErrTCPListenerOwnerUnverified
	}
	return nil
}

// VerifyWindowsTCPConnectionOwner binds a readiness probe to the PID that owns
// the server side of its established loopback connection.
func VerifyWindowsTCPConnectionOwner(ctx context.Context, processID int, bindAddress string, port uint16, clientAddress string, clientPort uint16) error {
	if ctx == nil || processID <= 0 || port == 0 || clientPort == 0 {
		return ErrTCPListenerOwnerUnverified
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	serverIP := net.ParseIP(bindAddress)
	clientIP := net.ParseIP(clientAddress)
	if serverIP == nil || clientIP == nil || !clientIP.IsLoopback() {
		return ErrTCPListenerOwnerUnverified
	}
	if serverV4 := serverIP.To4(); serverV4 != nil {
		clientV4 := clientIP.To4()
		if bindAddress != "127.0.0.1" || clientV4 == nil || !clientV4.IsLoopback() {
			return ErrTCPListenerOwnerUnverified
		}
		return verifyWindowsIPv4TCPConnectionOwner(ctx, processID, serverV4, port, clientV4, clientPort)
	}
	clientV6 := clientIP.To16()
	if bindAddress != "::1" || clientIP.To4() != nil || clientV6 == nil || !clientV6.IsLoopback() {
		return ErrTCPListenerOwnerUnverified
	}
	return verifyWindowsIPv6TCPConnectionOwner(ctx, processID, serverIP.To16(), port, clientV6, clientPort)
}

func verifyWindowsIPv4TCPConnectionOwner(ctx context.Context, processID int, serverAddress net.IP, serverPort uint16, clientAddress net.IP, clientPort uint16) error {
	table, err := queryWindowsTCPTable(ctx, ipHelperIPv4, tcpTableOwnerPIDAll)
	if err != nil {
		return err
	}
	rowOffset := int(unsafe.Offsetof(mibTCPTableOwnerPID{}.rows))
	rowSize := int(unsafe.Sizeof(mibTCPRowOwnerPID{}))
	count, ok := windowsTCPTableCount(table, rowOffset, rowSize)
	if !ok {
		return errors.New("Windows IPv4 TCP connection table is malformed")
	}
	matchingRows := 0
	ownerMatches := false
	for index := uint32(0); index < count; index++ {
		row := table[rowOffset+int(index)*rowSize:]
		if binary.LittleEndian.Uint32(row[0:4]) != tcpStateEstablished ||
			!equalBytes(row[4:8], serverAddress) || binary.BigEndian.Uint16(row[8:10]) != serverPort ||
			!equalBytes(row[12:16], clientAddress) || binary.BigEndian.Uint16(row[16:18]) != clientPort {
			continue
		}
		matchingRows++
		ownerMatches = binary.LittleEndian.Uint32(row[20:24]) == uint32(processID)
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if matchingRows != 1 || !ownerMatches {
		return ErrTCPListenerOwnerUnverified
	}
	return nil
}

func verifyWindowsIPv6TCPConnectionOwner(ctx context.Context, processID int, serverAddress net.IP, serverPort uint16, clientAddress net.IP, clientPort uint16) error {
	table, err := queryWindowsTCPTable(ctx, ipHelperIPv6, tcpTableOwnerPIDAll)
	if err != nil {
		return err
	}
	rowOffset := int(unsafe.Offsetof(mibTCP6TableOwnerPID{}.rows))
	rowSize := int(unsafe.Sizeof(mibTCP6RowOwnerPID{}))
	count, ok := windowsTCPTableCount(table, rowOffset, rowSize)
	if !ok {
		return errors.New("Windows IPv6 TCP connection table is malformed")
	}
	matchingRows := 0
	ownerMatches := false
	for index := uint32(0); index < count; index++ {
		row := table[rowOffset+int(index)*rowSize:]
		if binary.LittleEndian.Uint32(row[48:52]) != tcpStateEstablished ||
			!equalBytes(row[0:16], serverAddress) || binary.BigEndian.Uint16(row[20:22]) != serverPort ||
			!equalBytes(row[24:40], clientAddress) || binary.BigEndian.Uint16(row[44:46]) != clientPort {
			continue
		}
		matchingRows++
		ownerMatches = binary.LittleEndian.Uint32(row[52:56]) == uint32(processID)
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if matchingRows != 1 || !ownerMatches {
		return ErrTCPListenerOwnerUnverified
	}
	return nil
}

func queryWindowsTCPTable(ctx context.Context, addressFamily, tableClass uint32) ([]byte, error) {
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var size uint32
		status, _, _ := getExtendedTCPTableProc.Call(
			0, uintptr(unsafe.Pointer(&size)), 0, uintptr(addressFamily), uintptr(tableClass), 0,
		)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if status == 0 && size == 0 {
			return make([]byte, 4), nil
		}
		if status != uintptr(errorInsufficientBuffer) || size < 4 || size > maxWindowsTCPTableBytes {
			return nil, fmt.Errorf("GetExtendedTcpTable size query failed with status %d", status)
		}
		table := make([]byte, size)
		status, _, _ = getExtendedTCPTableProc.Call(
			uintptr(unsafe.Pointer(&table[0])), uintptr(unsafe.Pointer(&size)), 0,
			uintptr(addressFamily), uintptr(tableClass), 0,
		)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if status == uintptr(errorInsufficientBuffer) {
			continue
		}
		if status != 0 {
			return nil, fmt.Errorf("GetExtendedTcpTable failed with status %d", status)
		}
		if size < 4 || size > uint32(len(table)) {
			return nil, errors.New("GetExtendedTcpTable returned an invalid byte count")
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return table[:size], nil
	}
	return nil, errors.New("GetExtendedTcpTable changed size during three bounded reads")
}

func windowsTCPTableCount(table []byte, rowOffset, rowSize int) (uint32, bool) {
	if rowOffset < 4 || rowSize <= 0 || len(table) < rowOffset {
		return 0, false
	}
	count := binary.LittleEndian.Uint32(table[:4])
	if uint64(rowOffset)+uint64(count)*uint64(rowSize) > uint64(len(table)) {
		return 0, false
	}
	return count, true
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
