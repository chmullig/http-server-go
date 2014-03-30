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

    //open an mdb-lookup connection, and wrap it in a bufio R/W
    mdbconn, err := net.Dial("tcp", fmt.Sprintf("%s:%s", lookup_host, lookup_port))
    if err != nil {
        os.Exit(1)
    }
    mdbrw := bufio.NewReadWriter(bufio.NewReader(mdbconn), bufio.NewWriter(mdbconn))

    //Listen on the given port
    ln, err := net.Listen("tcp", fmt.Sprintf(":%s", listen_port))
    if err != nil {
        os.Exit(1)
    }

    //accept connections, and hand them off to a goroutine
    for {
        conn, err := ln.Accept()
        if err != nil {
            fmt.Println(err)
            continue
        }
        go handleConnection(conn, web_root, mdbrw)
    }
}

func prepHeaders(conn net.Conn, request string, code int) (headers []byte){
    host, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
    fmt.Printf("%s \"%s\" %d %s\n", host, request, code, statusString(code))
    headers = []byte(fmt.Sprintf("HTTP/1.0 %d %s\r\n\r\n", code, statusString(code)))
    return headers
}

//Just prepare an error page
func prepErrorPage(code int) (body []byte) {
    summary := fmt.Sprintf("%d %s", code, statusString(code))
    body = []byte(fmt.Sprintf("<title>%s</title><h1>%s</h1>", summary, summary))
    return body
}

//Send an error page and close the connection
func sendErrorPage(conn net.Conn, rw *bufio.ReadWriter, request string, code int, body []byte) {
        rw.Write(prepHeaders(conn, request, code))
        if body == nil {
            body = prepErrorPage(code)
        }
        rw.Write(body)
        rw.Flush()
        conn.Close()
}

func handleConnection(conn net.Conn, root string, mdbrw *bufio.ReadWriter) {
    rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

    //Read and slightly parse the headers
    scanner := bufio.NewScanner(conn)
    scanner.Scan()
    request_line := scanner.Text()
    for scanner.Scan() {
        txt := scanner.Text()
        if len(txt) < 1 { break; }
    }
    request := strings.Split(request_line, " ")

    //Make sure it's at least a slightly valid HTTP request
    if len(request) < 3 {
        sendErrorPage(conn, rw, request_line, 400, nil)
        return
    } else if request[len(request)-1] != "HTTP/1.0" && request[len(request)-1] != "HTTP/1.1" {
        fmt.Println(request[len(request)-1])
        sendErrorPage(conn, rw, request_line, 402, nil)
        return
    } else if request[0] != "GET" {
        sendErrorPage(conn, rw, request_line, 501, nil)
        return
    }

    //Take a look at what they requested, and see if we can provide it
    file := strings.Join(request[1:len(request)-1], " ")
    fn := path.Join(root, file)
    rdr, rdr_err := os.Open(fn)
    fi, fi_err := os.Stat(fn)

    if fi_err != nil {
        fmt.Println(fi_err)
        sendErrorPage(conn, rw, request_line, 404, nil)
        return
    } else if fi.IsDir() {
        //We're a directory, let's see if there's an index.html we can serve,
        //otherwise send the directory listing
        if _, err := os.Stat(path.Join(fn, "index.html")); os.IsNotExist(err) {
            rw.Write(prepHeaders(conn, request_line, 200))
            rw.Write([]byte("<body><ul>\n"))
            files, _ := rdr.Readdirnames(0)
            for _, name := range files {
                row := []byte(fmt.Sprintf("<li><a href=\"%s%s\">%s</a></li>\n", file, name, name));
                rw.Write(row)
            }
            rw.Write([]byte("</ul></body>\n"))
            rw.Flush()
            conn.Close()
            return
        } else {
            fn = path.Join(root, file, "index.html")
            rdr, rdr_err = os.Open(fn)
        }
    }

    //Try to open file, check a few common error conditions
    if os.IsPermission(rdr_err) {
        sendErrorPage(conn, rw, request_line, 403, nil)
        return
    } else if os.IsNotExist(rdr_err) {
        sendErrorPage(conn, rw, request_line, 404, nil)
        return
    } else if rdr_err != nil {
        sendErrorPage(conn, rw, request_line, 500, nil)
        return
    }

    //At long last, everything is ok so let's send the file
    rw.Write(prepHeaders(conn, request_line, 200))
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
