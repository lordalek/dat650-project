package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type MiningResults struct {
	MiningPower float64
	Fee         float64
	Blocks      float64
	UncleBlocks float64
	counter     float64
}

func NewMiningResults(miningPower, fee, blocks, uncleBlocks string) *MiningResults {
	r := &MiningResults{}
	formatedMiningPower, _ := strconv.ParseFloat(miningPower, 64)
	r.MiningPower = formatedMiningPower
	r.AddResults(fee, blocks, uncleBlocks)
	return r
}

func (s *MiningResults) AddResults(fee, blocks, uncleBlocks string) {
	formatedFee, _ := strconv.ParseFloat(fee, 64)
	formatedBlocks, _ := strconv.ParseFloat(blocks, 64)
	formatedUncleBlocks, _ := strconv.ParseFloat(uncleBlocks, 64)
	if s.counter == 0 {
		s.Fee += formatedFee
		s.Blocks += formatedBlocks
		s.UncleBlocks += formatedUncleBlocks
	} else {
		s.Fee = s.Fee*s.counter/(s.counter+1) + formatedFee/(s.counter+1)
		s.Blocks = s.Blocks*s.counter/(s.counter+1) + formatedBlocks/(s.counter+1)
		s.UncleBlocks = s.UncleBlocks*s.counter/(s.counter+1) + formatedUncleBlocks/(s.counter+1)
	}

	s.counter++
}

func main() {
	files := []string{}
	pathS, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	filepath.Walk(pathS, func(path string, f os.FileInfo, _ error) error {
		if !f.IsDir() {
			r, err := regexp.MatchString(".json.csv", f.Name())
			if err == nil && r {
				files = append(files, f.Name())
			}
		}
		return nil
	})

	for _, entry := range files {
		miners := make(map[string]*MiningResults)
		if content, err := os.ReadFile(entry); err != nil {
			panic(err)
		} else {
			// fmt.Println(string(content))
			splitData := strings.Split(string(content), "\n")
			for _, foo := range splitData {
				bar := strings.Split(foo, ",")
				if "minerID" == bar[0] || len(foo) == 0 {
					continue
				}

				if _, f := miners[bar[0]]; f == true {
					//upsert
					miners[bar[0]].AddResults(bar[2], bar[3], bar[4])
				} else {
					//insert
					miners[bar[0]] = NewMiningResults(bar[1], bar[2], bar[3], bar[4])
				}
			}
		}
		for key, baz := range miners {
			fmt.Printf("key %s \n\tpower %f\n\tfee %f\n\tblocks %f\n\tuncles %f\n", key, baz.MiningPower, baz.Fee, baz.Blocks, baz.UncleBlocks)
		}

		f, err := os.OpenFile(fmt.Sprintf("output-%s", entry),
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Println(err)
		}
		defer f.Close()
		for i := 0; i < 100; i++ {
			if _, found := miners[fmt.Sprintf("m%d", i)]; found == false {
				if _, err := f.WriteString(fmt.Sprintf("%s,%d,%d,%d,%d\n", fmt.Sprintf("m%d", i), 0, 0, 0, 0)); err != nil {
					log.Println(err)
				}
			} else {
				baz := miners[fmt.Sprintf("m%d", i)]
				if _, err := f.WriteString(fmt.Sprintf("%s,%f,%f,%f,%f\n", fmt.Sprintf("m%d", i), baz.MiningPower, baz.Fee, baz.Blocks, baz.UncleBlocks)); err != nil {
					log.Println(err)
				}
			}
		}

		if s, found := miners["s0"]; found == true {
			if _, err := f.WriteString(fmt.Sprintf("%s,%f,%f,%f,%f\n", "s0", s.MiningPower, s.Fee, s.Blocks, s.UncleBlocks)); err != nil {
				log.Println(err)
			}
		} else {
			if _, err := f.WriteString(fmt.Sprintf("%s,%d,%d,%d,%d\n", "s0", 0, 0, 0, 0)); err != nil {
				log.Println(err)
			}
		}
	}
}
