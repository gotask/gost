package stutil

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"bufio"

	"golang.org/x/crypto/ssh"
)

func SSHNew(user, password, ip_port string) (*ssh.Client, error) {
	PassWd := []ssh.AuthMethod{ssh.Password(password)}
	Conf := ssh.ClientConfig{User: user, Auth: PassWd}
	return ssh.Dial("tcp", ip_port, &Conf)
}

func SSHFile(user, keyFilePath, ip_port string) (*ssh.Client, error) {
	keyFileContents, err := ioutil.ReadFile(keyFilePath)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(keyFileContents)
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
	}

	return ssh.Dial("tcp", ip_port, config)
}

func SSHExeCmd(client *ssh.Client, cmd string) (string, error) {
	// Each ClientConn can support multiple interactive sessions,
	// represented by a Session.
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("Failed to create session: " + err.Error())
	}
	defer session.Close()

	// Once a Session is created, you can execute a single command on
	// the remote side using the Run method.
	var b bytes.Buffer
	session.Stdout = &b
	if err := session.Run(cmd); err != nil {
		return "", err
	}

	return b.String(), nil
}

func SSHScp2Remote(client *ssh.Client, localFile string, remotePathOrFile string, process chan float32) (err error) {
	defer func() {
		if process != nil {
			close(process)
		}
	}()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("Failed to create session: " + err.Error())
	}
	defer session.Close()

	writer, err := session.StdinPipe()
	if err != nil {
		return err
	}

	file, err := os.Open(localFile)
	if err != nil {
		return err
	}
	info, _ := file.Stat()
	defer file.Close()

	err = session.Start(fmt.Sprintf("/usr/bin/scp -qrt %s", remotePathOrFile))
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		fmt.Fprintln(writer, "C0644", info.Size(), filepath.Base(localFile))
		//io.CopyN(writer, file, info.Size())

		tatolSize := float32(info.Size())
		var written float32
		buf := make([]byte, 32*1024)
		for {
			nr, er := file.Read(buf)
			if nr > 0 {
				nw, ew := writer.Write(buf[0:nr])
				if nw > 0 {
					written += float32(nw)
				}
				if ew != nil {
					err = ew
					break
				}
				if nr != nw {
					err = io.ErrShortWrite
					break
				}
			}
			if er == io.EOF {
				break
			}
			if er != nil {
				err = er
				break
			}

			select {
			case process <- written / tatolSize:
			default:
			}
		}

		fmt.Fprint(writer, "\x00")
		writer.Close()
		wg.Done()
	}()

	wg.Wait()
	e := session.Wait()
	if e != nil && err != nil {
		return fmt.Errorf("%s;%s", err.Error(), e.Error())
	} else if e != nil {
		return e
	}
	return
}

func SSHScp2Local(client *ssh.Client, remoteFile string, localFilePath string, process chan float32) (err error) {
	defer func() {
		if process != nil {
			close(process)
		}
	}()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	writer, err := session.StdinPipe()
	if err != nil {
		return err
	}

	reader, err := session.StdoutPipe()
	if err != nil {
		return err
	}

	err = session.Start("/usr/bin/scp -f " + remoteFile)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func(writer io.WriteCloser, reader io.Reader, wg *sync.WaitGroup) {
		defer wg.Done()

		successfulByte := []byte{0}

		//use a scanner for processing individual commands, but not files themselves
		scanner := bufio.NewScanner(reader)

	NEXT:
		// Send a null byte saying that we are ready to receive the data
		writer.Write(successfulByte)

		scanner.Scan()
		err = scanner.Err()
		if err != nil {
			return
		}
		//first line
		scpStartLine := scanner.Text()

		if scpStartLine == "" {
			return
		}

		// We want to first receive the command input from remote machine
		// e.g. C0644 113828 test.csv
		if scpStartLine[0] == 0x1 {
			goto NEXT //scp: xxx: not a regular file
		}

		scpStartLineArray := strings.Split(scpStartLine, " ")

		if len(scpStartLineArray) != 3 {
			err = fmt.Errorf(scpStartLine)
			return
		}

		//filePermission := scpStartLineArray[0]
		fileSize := scpStartLineArray[1]
		fileName := scpStartLineArray[2]
		fileName = strings.Replace(fileName, "\n", "", -1)

		fileS, _ := strconv.Atoi(fileSize)

		//fmt.Println("File with permissions: %s, File Size: %s, File Name: %s", filePermission, fileSize, fileName)

		// Confirm to the remote host that we have received the command line
		writer.Write(successfulByte)
		// Now we want to start receiving the file itself from the remote machine

		var file *os.File
		file, err = FileCreate(localFilePath + fileName)
		if err != nil {
			return
		}
		more := true
		var written int
		var bytesRead int
		for more {
			need := fileS - written
			bufSize := 32 * 1024
			if need < bufSize {
				bufSize = need
			}
			fileContents := make([]byte, bufSize)
			bytesRead, err = reader.Read(fileContents)
			if err != nil {
				if err == io.EOF {
					more = false
				} else {
					return
				}
			}
			writeLen := bytesRead
			if writeLen > need {
				writeLen = need
			}
			file.Write(fileContents[:writeLen])

			written = written + writeLen

			select {
			case process <- float32(written) / float32(fileS):
			default:
			}

			if written == fileS {
				//close file writer & check error
				err = file.Close()
				if err != nil {
					return
				}
				//get next byte from channel reader
				nb := make([]byte, 1)
				_, err = reader.Read(nb)
				if err != nil {
					return
				}
				goto NEXT
			}
		}
	}(writer, reader, &wg)

	wg.Wait()
	writer.Close()

	e := session.Wait()
	if e != nil && err != nil {
		return fmt.Errorf("%s;%s", err.Error(), e.Error())
	} else if e != nil {
		return e
	}
	return
}
