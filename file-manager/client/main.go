package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
)

const downloadFileDir = "../download_file_dir"

func isEOF(file *os.File) (bool, error) {
	currentOffset, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return false, fmt.Errorf("failed to get current offset: %v", err)
	}

	fileStat, err := file.Stat()
	if err != nil {
		return false, fmt.Errorf("failed to get file stats: %v", err)
	}

	return currentOffset >= fileStat.Size(), nil
}

func uploadFile() {
	var file_path string
	var file_name string
	fmt.Print("Write File Path : ")
	fmt.Scanln(&file_path)
	fmt.Print("Write File Name : ")
	fmt.Scanln(&file_name)

	originalFileName := file_path + "/" + file_name

	file, err := os.Open(originalFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// 파일 내용 첫 번째 100바이트 읽기
	fileContent := make([]byte, 100)
	n, err := file.Read(fileContent)
	if err != nil && err != io.EOF {
		log.Fatal(err)
	}
	fmt.Println("Read File Content:", string(fileContent[:n]))

	/*
		파일 포인터를 처음으로 되돌리기
		이 작업을 하지 않은 경우 io.Copy 시에 파일이 정상적으로 복사되지 않는다
	*/
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		log.Fatal(err)
	}

	/*
		multipart는 주로 HTTP 요청에서 파일을 업로드할 때 사용되는 표준 형식
			여러 파일이나 데이터를 동시에 받기 위해 사용된다
			아래 코드에선 writer 객체를 이용해 데이터를 저장하고 있다.
	*/
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	part, err := writer.CreateFormFile("file", originalFileName)
	if err != nil {
		log.Fatal(err)
	}

	/*
		io.Copy는 파일 내용을 요청 본문에 복사하는 함수이며 내부적으로 반복적으로 file.Read를 호출하며 데이터를 읽어낸다.
		읽은 후 EOF에 도달했을 때 리턴하며 당시 err는 nil이다.
		그렇기에 io.Copy 동작 전에 file read pointer eof 검사가 필요하다.
			eof 위치에서 io.Copy 호출하는 경우 err는 아니지만 파일의 복사가 정상적으로 이루어지지 않는다.
	*/
	var isEof bool
	isEof, err = isEOF(file)
	if err == nil && isEof {
		log.Fatal(err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		log.Fatal(err)
	}

	err = writer.WriteField("file_name", file_name)
	if err != nil {
		log.Fatal(err)
	}

	err = writer.WriteField("file_path", file_path)
	if err != nil {
		log.Fatal(err)
	}

	err = writer.Close()
	if err != nil {
		log.Fatal(err)
	}

	request, err := http.NewRequest("POST", "http://localhost:8080/upload", &requestBody)
	if err != nil {
		log.Fatal(err)
	}

	request.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	fmt.Println("Response Status:", response.Status)
}

func searchFile() {
	response, err := http.Get("http://localhost:8080/search")
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	//
	/*
		[] : 슬라이스(slice), 동적으로 크기를 변경할 수 있는 배열
		map[string]interface{} : map 자료형, key-value 저장하며 여기서 string은 key의 type,
																													interface{}는 value의 모든 타입을 허용한다는 의미

		json.Unmarshal()을 통해 json 형식으로 받은 데이터를 go 데이터 형식으로 변환
	*/
	var data []map[string]interface{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("")
	for _, item := range data {
		fmt.Println("File ID:", item["_id"])
		fmt.Println("File Name:", item["originalFilename"])
		fmt.Println("File Update Date:", item["uploadedAt"])
		fmt.Println("")
	}
}

func downloadFile() {
	// 미구현
}

func deleteFile() {
	var delete_file_name string
	fmt.Print("Write Delete File Id : ")
	fmt.Scanln(&delete_file_name)

	formData := url.Values{}
	formData.Set("deleteFileId", delete_file_name)

	resp, err := http.PostForm("http://localhost:8080/delete", formData)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()

	// 서버 응답을 읽습니다.
	var responseBuffer bytes.Buffer
	_, err = responseBuffer.ReadFrom(resp.Body)
	if err != nil {
		fmt.Println("Error reading response:", err)
		return
	}

	// 응답 출력
	fmt.Println("Response from server:", responseBuffer.String())
}

func main() {
	var file_n rune

	for {
		fmt.Println("What do you want for about file? ")
		fmt.Println("1. File Upload")
		fmt.Println("2. File Search")
		fmt.Println("3. File Download")
		fmt.Println("4. File Delete")
		fmt.Println("If you want to exit this job, press x")
		fmt.Print("job : ")
		fmt.Scanln(&file_n)

		if file_n == 'x' {
			break
		} else {
			var num int = int(file_n)
			if num == 1 {
				uploadFile()
			} else if num == 2 {
				searchFile()
			} else if num == 3 {
				downloadFile()
			} else if num == 4 {
				deleteFile()
			} else {
				fmt.Println("Take Care Number")
				continue
			}
		}
	}
}
