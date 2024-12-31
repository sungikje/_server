package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const uploadFileDir = "../file_path/upload_file_dir"
const deletedFileDir = "../file_path/deleted_file_dir"

/*
필요한 필드 구조체 정의
여기서 bson은 Binary JSON의 약자로 json을 확장한 이진 형식이다.
MongoDB와 같은 NoSQL 데이터베이스에서 데이터를 저장하고 전송하는데 사용된다.
*/
type Document struct {
	OriginalFilename string `bson:"originalFilename"`
	StoredFileName   string `bson:"storedFileName"`
}

// 전역 변수로 MongoDB 클라이언트 선언
var client *mongo.Client

func getFileSize(file multipart.File) (int64, error) {
	// 더 큰 버퍼 크기 (예: 1MB)
	buf := make([]byte, 1024*1024) // 1MB 버퍼 크기
	var size int64

	for {
		n, err := file.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
		size += int64(n)
	}

	return size, nil
}

// 파일 업로드 처리
func uploadFile(w http.ResponseWriter, r *http.Request) {
	// 제한된 크기(여기서는 80MB)의 multipart/form-data 형식 데이터를 파싱한다
	err := r.ParseMultipartForm(80 << 20)
	if err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// file, header, err := r.FormFile("file")
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Unable to read file", http.StatusBadRequest)
		return
	}

	fileSize, err := getFileSize(file)
	if err != nil {
		log.Fatal(err)
	}

	defer file.Close()

	originalFileName := r.FormValue("file_name")

	currentTime := time.Now()
	formattedTime := currentTime.Format("20060102150405")
	storedFileName := formattedTime + "_" + originalFileName // Client로부터 전송 받은 파일 이름에 초까지의 시간을 string 형태로 prefix

	filePath := filepath.Join(uploadFileDir, storedFileName)
	outFile, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Unable to save file", http.StatusInternalServerError)
		return
	}
	defer outFile.Close()

	/*
		파일 포인터를 처음으로 이동
			해당 작업이 없는 경우 파일 Read Pointer가 EOF를 가리키며 정상적으로 파일을 읽지 못했다.
			이유는 위에 r.FormFile() 함수 사용 시 파일을 정상적으로 읽어낸 후 EOF에 Read 포인터가 위치한 상태였기 때문이다.
	*/
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		http.Error(w, "Unable to seek file", http.StatusInternalServerError)
		return
	}

	_, err = io.Copy(outFile, file)
	if err != nil {
		http.Error(w, "Unable to write file", http.StatusInternalServerError)
		return
	}

	insertFileMetadata(originalFileName, storedFileName, filePath, fileSize, currentTime)

	// 응답
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "File uploaded successfully")
}

// 파일 검색 처리 (업로드된 파일 목록)
func searchFile(w http.ResponseWriter, r *http.Request) {
	err := client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}

	collection := client.Database("file").Collection("metadata")

	// 모든 문서 조회
	cursor, err := collection.Find(context.Background(), bson.D{})
	if err != nil {
		http.Error(w, "Unable to retrieve documents", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	/*
		문서를 슬라이스에 저장
			bson.M은 map[string]interface{} 타입의 별칭

		cursor : MongoDB 쿼리를 받아오는 커서, Decode를 이용해 Go 데이터 형식으로 변환
		context.Background() : Go에서 제공하는 context 객체, 주로 함수나 요청 처리의 컨텍스트를 나타낸다.
			컨텍스트는 간단하게 작업 추적 관리하는 역할, 컨텍스트 객체는 작업의 수명 주기를 관리하고, 작업 취소, 타임아웃, 데드라인 등을 설정할 수 있게 해줌
	*/
	var documents []bson.M
	for cursor.Next(context.Background()) {
		var document bson.M
		if err := cursor.Decode(&document); err != nil {
			http.Error(w, "Unable to decode document", http.StatusInternalServerError)
			return
		}
		documents = append(documents, document)
	}

	if err := cursor.Err(); err != nil {
		http.Error(w, "Error reading from cursor", http.StatusInternalServerError)
		return
	}

	// JSON 형식으로 응답
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(documents)
}

// 파일 다운로드 처리
func downloadFile(w http.ResponseWriter, r *http.Request) {
	// 미구현
}

/*
파일 삭제 처리

	삭제 시 파일은 휴지통으로 이동, Document는 deleteAt 갱신 및 deleteTf true 업데이트
*/
func deleteFile(w http.ResponseWriter, r *http.Request) {
	// 요청 본문에서 form 데이터를 파싱합니다.
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	deleteFileId := r.FormValue("deleteFileId")
	if deleteFileId == "" {
		fmt.Println("Deleted File ID is Null")
		return
	}
	fmt.Print("Deleted File ID : ")
	fmt.Println(deleteFileId)

	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}

	collection := client.Database("file").Collection("metadata")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// _id로 조회
	objectID, err := primitive.ObjectIDFromHex(deleteFileId)
	if err != nil {
		log.Fatalf("Invalid ObjectID: %v", err)
	}

	projection := bson.M{
		"originalFilename": 1,
		"storedFileName":   1,
	}

	/*
		`_id`로 문서 조회 (프로젝션 사용)
			프로젝션은 쿼리의 결과로 반환되는 필드를 선택적으로 제한하는 방법, 특정 필드만을 선택해서 결과로 반환받는다.
	*/
	var result Document
	err = collection.FindOne(ctx, bson.M{"_id": objectID}, options.FindOne().SetProjection(projection)).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			fmt.Println("No document found with the specified _id")
		} else {
			log.Fatalf("Failed to find document: %v", err)
		}
		return
	}

	// 결과를 다른 변수에 저장하여 사용
	originalFilename := result.OriginalFilename
	storedFileName := result.StoredFileName

	// 업데이트 쿼리 실행
	currentTime := time.Now()
	formattedTime := currentTime.Format("20060102150405")
	filter := bson.M{"_id": objectID}
	update := bson.M{
		"$set": bson.M{
			"deletedTf": "true",
			"deletedAt": currentTime,
		},
	}
	result_update, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Fatal(err)
	}

	oldPath := uploadFileDir + "/" + storedFileName                          // 이동할 파일의 경로
	newPath := deletedFileDir + "/" + formattedTime + "_" + originalFilename // 이동할 파일의 대상 경로
	err = os.Rename(oldPath, newPath)
	if err != nil {
		fmt.Printf("Failed to move file: %v\n", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Printf("Successfully deleted %d document(s).\n", result_update)
}

func connectMongoDB() {
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	// 여기서 client := 이용하는 경우 내부 스코프로 한정, = 사용해야되며 err도 미리 선언 필요
	var err error
	client, err = mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}
	// defer client.Disconnect(context.Background())
	fmt.Println("Successfully connected to MongoDB!")
}

func insertFileMetadata(fileName string, storedFileName string, filePath string, fileSize int64, currentTime time.Time) {
	// MongoDB 연결 확인
	err := client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}

	// 'test' 데이터베이스와 'users' 컬렉션 참조
	collection := client.Database("file").Collection("metadata")
	// 결과 출력
	// fmt.Println("Formatted Date:", formattedDate)

	user := bson.D{
		{"originalFilename", fileName},
		{"storedFileName", storedFileName},
		{"fileSize", fileSize},
		{"path", filePath},
		{"uploadedAt", currentTime},
		{"deletedAt", ""},
		{"deletedTf", false},
	}

	// 데이터 삽입
	insertResult, err := collection.InsertOne(context.Background(), user)
	if err != nil {
		log.Fatal(err)
	}

	// 삽입된 데이터의 ID 출력
	fmt.Printf("Inserted document with ID: %v\n", insertResult.InsertedID)
}

func main() {
	// 업로드 파일을 저장할 디렉토리 확인
	err := os.MkdirAll(uploadFileDir, os.ModePerm)
	if err != nil {
		log.Fatal("Unable to create file directory:", err)
	}

	connectMongoDB()

	// HTTP 핸들러 등록
	http.HandleFunc("/upload", uploadFile)
	http.HandleFunc("/search", searchFile)
	http.HandleFunc("/download", downloadFile)
	http.HandleFunc("/delete", deleteFile)

	// 서버 시작
	fmt.Println("Server is listening on http://localhost:8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("Error starting server:", err)
	}

}
