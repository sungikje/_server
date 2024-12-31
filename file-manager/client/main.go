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
	// fmt.Printf("Copy bytes: %d\n", copyBytes) // copy bytes가 0이면 문제

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

	var data []map[string]interface{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("")
	for _, item := range data {
		// 예시: "name"과 "age"만 출력
		fmt.Println("File ID:", item["_id"])
		fmt.Println("File Name:", item["originalFilename"])
		fmt.Println("File Update Date:", item["uploadedAt"])
		fmt.Println("")
	}
}

func downloadFile() {
	var download_path string
	var download_file_name string
	var download_file_id int
	fmt.Print("Write Download File Path : ")
	fmt.Scanln(&download_path)
	fmt.Print("Write Download File Name : ")
	fmt.Scanln(&download_file_name)
	fmt.Print("Write Download File ID : ")
	fmt.Scanln(&download_file_id)

	// 서버에서 다운로드할 파일 URL
	url := "http://localhost:8080/download" + string(download_file_id)

	// GET 요청을 보내서 파일 다운로드
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	// 로컬 파일로 저장하기 위한 파일 생성
	out, err := os.Create(download_path + download_file_name)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	// 서버에서 받은 파일 내용을 로컬 파일에 저장
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("File downloaded successfully")
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
