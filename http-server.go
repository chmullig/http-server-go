package main

import (
    "fmt"
    "os"
    "strings"
    "path"
    "net"
    "bufio"
)

func main() {
    if len(os.Args) != 5 {
        fmt.Printf("%s <server_port> <web_root> <mdb-lookup-host> <mdb-lookup-port>", os.Args[0])
        os.Exit(1)
    }

    listen_port := os.Args[1]
    web_root := os.Args[2]
    lookup_host := os.Args[3]
    lookup_port := os.Args[4]

    mdbconn, err := net.Dial("tcp", fmt.Sprintf("%s:%s", lookup_host, lookup_port))
    if err != nil {
        os.Exit(1)
    }
    mdbrw := bufio.NewReadWriter(bufio.NewReader(mdbconn), bufio.NewWriter(mdbconn))

    ln, err := net.Listen("tcp", fmt.Sprintf(":%s", listen_port))
    if err != nil {
        os.Exit(1)
    }

    for {
        conn, err := ln.Accept()
        if err != nil {
            fmt.Println(err)
            continue
        }
        go handleConnection(conn, web_root, mdbrw)
    }
}


func prepErrorPage(code int) (body []byte) {
    summary := fmt.Sprintf("%d %s", code, statusString(code))
    body = []byte(fmt.Sprintf("<title>%s</title><h1>%s</h1>", summary, summary))
    return body
}


func sendErrorPage(conn net.Conn, rw *bufio.ReadWriter, code int, body []byte) {
        fmt.Printf("sending a %d\n", code)
        if body == nil {
            body = prepErrorPage(code)
        }
        headers := []byte(fmt.Sprintf("HTTP/1.0 %d %s\r\n\r\n", code, statusString(code)))
        rw.Write(headers)
        rw.Write(body)
        rw.Flush()
        conn.Close()
}

func handleConnection(conn net.Conn, root string, mdbrw *bufio.ReadWriter) {
    fmt.Printf("Handling connection from %s\n", conn.RemoteAddr())

    rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

    scanner := bufio.NewScanner(conn)
    scanner.Scan()

    request_line := scanner.Text()

    for scanner.Scan() {
        txt := scanner.Text()
        if len(txt) < 1 { break; }
    }
    request := strings.Split(request_line, " ")
    fmt.Println(request_line)

    if false {
        sendErrorPage(conn, rw, 400, nil)
        return
    } else if len(request) < 3 {
        sendErrorPage(conn, rw, 400, nil)
        return
    } else if request[len(request)-1] != "HTTP/1.0" && request[len(request)-1] != "HTTP/1.1" {
        fmt.Println(request[len(request)-1])
        sendErrorPage(conn, rw, 402, nil)
        return
    } else if request[0] != "GET" {
        sendErrorPage(conn, rw, 501, nil)
        return
    }

    file := strings.Join(request[1:len(request)-1], " ")
    fn := path.Join(root, file)
    fi, err := os.Stat(fn)
    if err != nil {
        fmt.Println(err)
        sendErrorPage(conn, rw, 404, nil)
        return
    } else if fi.IsDir() {
        //body = []byte("this is a directory...")
    }

    rdr, err := os.Open(fn)
    if err != nil {
        sendErrorPage(conn, rw, 500, nil)
        return
    }

    code := 200
    headers := []byte(fmt.Sprintf("HTTP/1.0 %d %s\r\n\r\n", code, statusString(code)))
    rw.Write(headers)
    for {
        buffer := make([]byte, 4096)
        bytesRead, err := rdr.Read(buffer)
        if bytesRead == 0 || err != nil {
            break
        }
        bytesWritten, _ := rw.Write(buffer[:bytesRead])
        if bytesRead != bytesWritten {
            break
        }
    }

    rw.Flush()
    conn.Close()
}


func statusString(code int) (msg string) {
    msgDb := map[int]string{
        200: "OK",
        304: "Not Modified",
        400: "Bad Request",
        401: "Unauthorized",
        403: "Forbidden",
        404: "Not Found",
        405: "Method Not Allowed",
        418: "I'm a teapot",
        500: "Internal Server Error",
        501: "Not Implemented",
        505: "HTTP Version Not Supported",
    }
    msg = msgDb[code]
    return msg
}
