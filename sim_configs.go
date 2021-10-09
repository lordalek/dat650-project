package main

import (
	"encoding/json"
	"os"
	"fmt"
)

type config struct {
	Runs		int	//default 20
	Time		int	//default 10^7
	Miners		int	//default 100
	MaxUncles	int	//limit of uncles included per block: min. 0, default max 2
	PowerScaling	float64	//number equal to or greater than 1.0, miner n will have pS^n mining power
	//PowerRandomness	float64	//top of random range of power, bottom is 1.0
	SelfishMiners	int	//one or zero, more possible but out of scope
	SelfishDelay	int	//how many rounds does a selfish miner wait before publishing a block?
	SelfishPower	float64	//percentile of regular miners the selfish miner has more mining power than, (0,1)
}

func main() {
	savePath := "./config/"
	runs := []int{20}
	time := []int{100000000}
	miners := []int{100}
	uncles := []int{0,2}
	powerScaling := []float64{1.0, 1.2}
	selfishMiners := []int{0,1}
	selfishDelay := []int{3,10,30,100}
	selfishPower := []float64{0.1,0.5,0.9}
	conf := config{}
	i := 0
	for _, r := range runs {
		for _, t := range time {
			for _, m := range miners {
				for _, u := range uncles {
					for _, p := range powerScaling {
						for _, s := range selfishMiners {
							if s > 0 {
								for _, sd := range selfishDelay {
									for _, sp := range selfishPower {
										confName := fmt.Sprintf("runs_%d_time_%d_miners_%d_uncles_%d_scaling_%f_selfish_%d_sdelay_%d_spower_%f",r,t,m,u,p,s,sd,sp)
										conf = config{r, t, m, u, p, s, sd, sp}
										jconf, err := json.Marshal(conf)
										if err == nil {
											file, _ := os.Create(fmt.Sprintf("%sconfig_%s.json", savePath, confName))
											file.Write(jconf)
											i++
										}
									}
								}
							} else {
								confName := fmt.Sprintf("runs_%d_time_%d_miners_%d_uncles_%d_scaling_%f_selfish_%d_sdelay_%d_spower_%f",r,t,m,u,p,s,0,0.0)
								conf = config{r, t, m, u, p, 0, 0, 0}
								jconf, err := json.Marshal(conf)
								if err == nil {
									file, _ := os.Create(fmt.Sprintf("%sconfig_%s.json", savePath, confName))
									file.Write(jconf)
									i++
								}
							}
						}
					}
				}
			}
		}
	}
}
