package main

import (
	"crypto/sha1"
	"errors"
	"io"
	"log"
	"os"
//	"fmt"
	//"strings"
	"reflect"
)

type status int

const (
	// CREATED file created not written yet - clean file
	CREATED status = iota
	// PAUSED file writing is currently paused
	PAUSED status = iota
	// WRITING - file or part of file is written
	WRITING status = iota
)

//FileWriter is the struct containing information writing to a file
type FileWriter struct {
	Info     *InfoDict
	DataFile *os.File
	Status   status
}

//NewFileWriter Create initializes a new File Writer write to a particular file based on info
//in the Info dictionary
func NewFileWriter(tInfo *InfoDict, fileName string) FileWriter {
	var f FileWriter
	f.Info = tInfo

	file, err := os.Create(fileName)
	if err != nil {
		log.Fatal("Error creating file to write pieces\n", err)
	}
	if err := file.Truncate(int64(tInfo.Length)); err != nil {
		log.Fatal("Unable to create a file with enough space for torrent\n", err)
	}

	f.DataFile = file // f is now the file where data is to be written
	f.Status = CREATED
	return f
}

//Write writes to the file specified in the FileWriter created before
func (f *FileWriter) Write(data []byte, index int) error {
	if f == nil {
		return errors.New("Undefined FileWriter\n")
	}

	if f.Status == PAUSED {
		return errors.New("File writing is paused.\n")
	}

	//check the sha1 hash
	//if f.checkSHA1(data, index) == false {
	//	return errors.New("Data SHA1 does not match piece SHA1\n")
	//}

	_, err := f.DataFile.WriteAt(data, int64(index*f.Info.PieceLength))
	//fmt.Println(err)
	return err
}

func (f *FileWriter) checkSHA1(data []byte, index int) bool {
	// compute the hash of data
	hash := sha1.New()
	io.WriteString(hash, string(data))
	dataHash := hash.Sum(nil)
	pieceHash := f.Info.Pieces[index*20 : (index+1)*20]
	
	return reflect.DeepEqual(dataHash, pieceHash)
}

// Delete destroys the file that has been created and the FileWriter
func (f *FileWriter) Delete() error {
	if f == nil {
		return errors.New("Undefined FileWriter\n")
	}
	err := os.Remove(f.Info.Name)
	if err != nil { // set f to nil only if error is nil
		f.DataFile.Close()
		f = nil
	}
	return err
}

// Finish releases all file resources and destroys the FileWriter before writing everything from buffer to disc
func (f *FileWriter) Finish() error {
	if f.Status == PAUSED {
		return errors.New("File Writing is paused")
	}
	f.DataFile.Close()
	f = nil
	return nil
}

func (f *FileWriter) Sync() error{

		if err := f.DataFile.Sync(); err != nil {
		return err
	}
	return nil

}

// Pause momentarily stops writing to the file - does not write until restarted
// and writes buffer to disc
func (f *FileWriter) Pause() error {
	if err := f.DataFile.Sync(); err != nil {
		return err
	}
	f.Status = PAUSED
	return nil
}

// Restart restarts writing to the file if paused else returns false
func (f *FileWriter) Restart() bool {
	if f.Status == PAUSED {
		f.Status = WRITING
		return true
	}
	return false
}
