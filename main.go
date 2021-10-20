package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

type SongsResponse struct {
	SongId  	string `json:"song_id"`
	ReleasedAt 	string `json:"released_at"`
	Duration    string `json:"duration"`
	Artist    	string `json:"artist"`
	Name    	string `json:"name"`
	Stats 		Stats  `json:"stats"`
}

type Stats struct {
	LastPlayedAt	int	`json:"last_played_at"`
	TimesPlayed 	int	`json:"times_played"`
	GlobalRank 		int `json:"global_rank"`
}

type SongsResponseError struct{
	Error	string `json:"error"`
}

const (
	TwentyFourHours		= 24
	YyyyMmDdLayout     	= "2006-01-02"
	YyyyMmLayout       	= "2006-01"
	SameApiCosts       	= 25
	SongsUriMonthly    	= "monthly"
	SongsUriDaily      	= "daily"
	SongsUriApiKey     	= "?api_key="
	SongsUriReleasedAt 	= "&released_at="
)

// getReleases responds with the list of all releases as JSON.
func getReleases(c *gin.Context) {
	from 	:= c.Query("from") // shortcut for c.Request.URL.Query().Get("from")
	until 	:= c.Query("until") // shortcut for c.Request.URL.Query().Get("until")
	artist 	:= c.Query("artist") // shortcut for c.Request.URL.Query().Get("artist")

	fmt.Println(from, until, artist)

	fromTime, err := time.Parse(YyyyMmDdLayout, from)

	if httpBadRequestError(err, c) {
		return
	}

	untilTime, err := time.Parse(YyyyMmDdLayout, until)

	if httpBadRequestError(err, c) {
		return
	}

	daysTotal := untilTime.Sub(fromTime).Hours() / TwentyFourHours

	fmt.Println(fromTime, untilTime, daysTotal)

	response, responseErr := getSongs(daysTotal, from, until, artist)

	fmt.Printf("response %vn", response)

	if responseErr != nil {
		c.JSON(http.StatusBadRequest, responseErr)
		log.Fatal(responseErr)
		return
	}

	c.JSON(http.StatusOK, response)
}

func httpBadRequestError(err error, c *gin.Context) bool {
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		log.Println(err.Error())
		return true
	}
	return false
}

func getSongs(daysTotal float64, from string, until string, artist string) (interface{}, *SongsResponseError) {
	var (
		songsResponses 		interface{}
		songsRequested		[]*SongsResponse
	)

	yyyyMmDdFrom, err := time.Parse(YyyyMmDdLayout, from)
	logErr(err)

	yyyyMmDdCurr := yyyyMmDdFrom
	yyyyMmDdUntil, err := time.Parse(YyyyMmDdLayout, until)
	logErr(err)

	/*
	if +25 days requested in month
		if less than total days in month, then parse days requested,
		else keep all songs
	else daily
	*/

	for yyyyMmDdUntil.Sub(yyyyMmDdCurr) > 0 {
		twentyFiveDaysFrom := yyyyMmDdCurr.AddDate(0, 0, SameApiCosts)
		twentyFiveDaysFrom.Month()

		if daysTotal > SameApiCosts && yyyyMmDdFrom.Month() == twentyFiveDaysFrom.Month() {
			songsResponse, err 		:= getSongsMonthly(yyyyMmDdCurr)
			songsResponseObj, ok 	:= songsResponseTypeAssertion(err, songsResponse)

			if !ok {
				logErr(errors.New("Failed to assert *SongsResponse"))

				songResponseErrObj, ok := songsResponses.(*SongsResponseError)

				if !ok {
					logErr(errors.New("Failed to assert []*SongsResponseError"))
				}

				return nil, songResponseErrObj
			}

			songsRequested = parseSongs(songsResponseObj, yyyyMmDdCurr, artist)

			yyyyMmDdCurr = yyyyMmDdCurr.AddDate(0, 1, 0)
		} else {
			songsResponse, err 		:= getSongsDaily(yyyyMmDdUntil, yyyyMmDdCurr)
			songsResponseObj, ok 	:= songsResponseTypeAssertion(err, songsResponse)

			if !ok {
				logErr(errors.New("Failed to assert *SongsResponse"))

				songResponseErrObj, ok := songsResponses.(*SongsResponseError)

				if !ok {
					logErr(errors.New("Failed to assert []*SongsResponseError"))
				}

				return nil, songResponseErrObj
			}

			songsRequested = parseSongs(songsResponseObj, yyyyMmDdCurr, artist)

			yyyyMmDdCurr = yyyyMmDdCurr.AddDate(0, 0, 1)
		}
	}

	return songsRequested, nil
}

func songsResponseTypeAssertion(err error, songsResponse interface{}) ([]*SongsResponse, bool) {
	if err != nil {
		logErr(err)
	}

	songsResponseObj, ok := songsResponse.([]*SongsResponse)

	fmt.Println("songsResponseObj %q", songsResponseObj)

	return songsResponseObj, ok
}

func parseSongs(songsResponseObj []*SongsResponse, yyyyMmDdCurr time.Time, artist string) ([]*SongsResponse) {
	var songsRequested []*SongsResponse

	// parse for requested songs
	for _, song := range songsResponseObj {
		fmt.Printf("song %T %V\n", song, song)

		songReleasedAt, err := time.Parse(YyyyMmDdLayout, song.ReleasedAt)
		logErr(err)

		if songReleasedAt.Sub(yyyyMmDdCurr) >= 0 {
			if artist != "" && song.Artist == artist {
				songsRequested = append(songsRequested, song)
			} else if artist == "" {
				songsRequested = append(songsRequested, song)
			}
		}
	}

	fmt.Printf("songsRequested %v\n", songsRequested)

	return songsRequested
}

func getSongsMonthly(yyyyMmDdCurr time.Time) (interface{}, error) {
	var (
		songsResponse  		[]*SongsResponse
		songsResponseError 	*SongsResponseError
		err 				error
	)

	url := os.Getenv("BASE_URL") + SongsUriMonthly + SongsUriApiKey + os.Getenv("API_KEY") + SongsUriReleasedAt +
		yyyyMmDdCurr.Format(YyyyMmLayout)

	response, err := http.Get(url)
	logErr(err)

	responseData, err := ioutil.ReadAll(response.Body)
	logErr(err)

	json.Unmarshal(responseData, &songsResponse)

	if err != nil {
		return songsResponseError, err
	}

	return songsResponse, nil
}

func getSongsDaily(yyyyMmDdUntil time.Time, yyyyMmDdCurr time.Time, ) (interface{}, error) {
	var (
		songsResponse		[]*SongsResponse
		songsResponseCurr	[]*SongsResponse
		songsResponseError 	*SongsResponseError
		err 				error
	)

	for yyyyMmDdUntil.Sub(yyyyMmDdCurr) > 0 {
		url := os.Getenv("BASE_URL") + SongsUriDaily + SongsUriApiKey + os.Getenv("API_KEY") + SongsUriReleasedAt +
			yyyyMmDdCurr.Format(YyyyMmDdLayout)

		response, err := http.Get(url)
		logErr(err)

		responseData, err := ioutil.ReadAll(response.Body)
		logErr(err)

		err = json.Unmarshal(responseData, &songsResponseCurr)

		if err != nil {
			return songsResponseError, err
		} else {
			for _, song := range songsResponseCurr {
				songsResponse = append(songsResponse, song)
			}
		}

		yyyyMmDdCurr = yyyyMmDdCurr.Add(time.Hour * TwentyFourHours)
	}

	return songsResponse, err
}

func logErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	router := gin.Default()
	router.GET("/releases", getReleases)

	router.Run(":8080")
}