package main

// https://medium.com/canal-tech/how-video-streaming-works-on-the-web-an-introduction-7919739f7e1

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/anacrolix/torrent"
)

func getLargestFile(torrentTorrent *torrent.Torrent) *torrent.File {
	var target *torrent.File
	var maxSize int64

	for _, file := range torrentTorrent.Files() {
		if maxSize < file.Length() {
			maxSize = file.Length()
			target = file
		}
	}

	return target
}

func main() {
	clientConfig := torrent.NewDefaultClientConfig()

	c, _ := torrent.NewClient(clientConfig)
	defer c.Close()

	http.HandleFunc("/streaming", func(w http.ResponseWriter, r *http.Request) {
		magnet, okQuery := r.URL.Query()["magnet"]

		if okQuery == false {
			fmt.Println("Error on got query")
		}

		t, _ := c.AddMagnet(magnet[0])
		<-t.GotInfo()
		t.DownloadAll()
		// set the 5% priority of the file
		largestFile := getLargestFile(t)
		firstPieceIndex := largestFile.Offset() * int64(t.NumPieces()) / t.Length()
		endPieceIndex := (largestFile.Offset() + largestFile.Length()) * int64(t.NumPieces()) / t.Length()
		for idx := firstPieceIndex; idx <= endPieceIndex*5/100; idx++ {
			t.Piece(int(idx)).SetPriority(torrent.PiecePriorityNow)
		}

		target := getLargestFile(t)
		reader := target.NewReader()

		// We read ahead 1% of the file continuously.
		reader.SetReadahead(target.Length() / 100)
		reader.SetResponsive()
		_, errSeek := reader.Seek(target.Offset(), io.SeekStart)

		if errSeek != nil {
			fmt.Println("Error on init seek")
		}

		http.ServeContent(w, r, target.DisplayPath(), time.Now(), reader)
	})

	http.Handle("/client/", http.StripPrefix("/client/", http.FileServer(http.Dir("./client"))))

	fmt.Println("🚀 Running at 8090")
	http.ListenAndServe(":8090", nil)
}

func readyForPlayback(torrentTorrent *torrent.Torrent) bool {
	info := torrentTorrent.Info()

	if info == nil {
		return false
	}

	percent := float64(torrentTorrent.BytesCompleted()) / float64(info.TotalLength()) * 100

	return percent > 5
}
