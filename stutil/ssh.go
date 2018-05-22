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

	file, err := os.Open(localFile)
	if err != nil {
		return err
	}
	info, _ := file.Stat()
	defer file.Close()

	err = session.Run(fmt.Sprintf("/usr/bin/scp -qrt %s", remotePathOrFile))
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		writer, _ := session.StdinPipe()
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

		// Send a null byte saying that we are ready to receive the data
		writer.Write(successfulByte)

	NEXT:
		// We want to first receive the command input from remote machine
		// e.g. C0644 113828 test.csv
		scpCommandArray := make([]byte, 100)
		var bytesRead int
		bytesRead, err = reader.Read(scpCommandArray)
		if err != nil /*&& err != io.EOF*/ {
			return
		}

		scpStartLine := string(scpCommandArray[:bytesRead])
		scpStartLineArray := strings.Split(scpStartLine, " ")

		if len(scpStartLineArray) != 3 {
			err = fmt.Errorf(scpStartLine)
			return
		}

		//fmt.Println("bytesRead:", bytesRead, scpStartLine)

		filePermission := scpStartLineArray[0]
		fileSize := scpStartLineArray[1]
		fileName := scpStartLineArray[2]
		fileName = strings.Replace(fileName, "\n", "", -1)

		fileS, _ := strconv.Atoi(fileSize)

		if !isValidPermission(filePermission) {
			err = fmt.Errorf(scpStartLine)
			return
		}

		//fmt.Println("File with permissions: %s, File Size: %s, File Name: %s", filePermission, fileSize, fileName)

		// Confirm to the remote host that we have received the command line
		writer.Write(successfulByte)
		// Now we want to start receiving the file itself from the remote machine
		fileContents := make([]byte, 32*1024)
		var file *os.File
		file, err = FileCreate(localFilePath + fileName)
		if err != nil {
			return
		}
		more := true
		var written int
		for more {
			bytesRead, err = reader.Read(fileContents)
			if err != nil {
				if err == io.EOF {
					more = false
				} else {
					return
				}
			}
			file.Write(fileContents[:bytesRead])
			writer.Write(successfulByte)

			written = written + bytesRead

			select {
			case process <- float32(written) / float32(fileS):
			default:
			}

			if written == fileS {
				//read end
				bytesRead, err = reader.Read(fileContents)
				goto NEXT
			}
		}
		err = file.Sync()
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

func isValidPermission(per string) bool {
	if !strings.HasPrefix(per, "C0") || len(per) != 5 {
		return false
	}
	s := per[2:3] //1 2 4
	if s < "1" || s > "7" {
		return false
	}
	s = per[3:4] //1 2 4
	if s < "1" || s > "7" {
		return false
	}
	s = per[4:] //1 2 4
	if s < "1" || s > "7" {
		return false
	}
	return true
}
