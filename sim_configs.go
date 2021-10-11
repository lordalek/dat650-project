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
	MaxDepth	int	//how old can an uncle be before all its value diminishes?
	UncleDivisor	float64	//static divisor on uncle reward
	NephewReward	float64	//portion of a block reward given to nephew per uncle included
	SelfishMiners	int	//one or zero, more possible but out of scope
	SelfishDelay	int	//how many rounds does a selfish miner wait before publishing a block?
	SelfishPower	float64	//percentile of regular miners the selfish miner has more mining power than, (0,1)
}

func main() {
	savePath := "./config_uncleOptions/"
	runs := []int{20}
	time := []int{100000000}
	miners := []int{100}
	uncles := []int{2}//, 5, 32}
	powerScaling := []float64{1.2}
	maxDepth := []int{7,3,15}
	uncleDivisor := []float64{1.0, 2.0}
	nephewReward := []float64{1/32}
	selfishMiners := []int{0,1}
	selfishDelay := []int{10,30}
	selfishPower := []float64{0.1,0.9}
	conf := config{}
	i := 0
	for _, r := range runs {
		for _, t := range time {
			for _, m := range miners {
				for _, u := range uncles {
					for _, p := range powerScaling {
						for _, md := range maxDepth {
							for _, ud := range uncleDivisor {
								if ud == 1.0 {
									for _, nr := range nephewReward {
										for _, s := range selfishMiners {
											if s > 0 {
												for _, sd := range selfishDelay {
													for _, sp := range selfishPower {
														confName := fmt.Sprintf("uncles_%d_scaling_%f_maxdepth_%d_selfish_%d_sdelay_%d_spower_%f",u,p,md,s,sd,sp)
														conf = config{r, t, m, u, p, md, 1.0, nr, s, sd, sp}
														jconf, err := json.Marshal(conf)
														if err == nil {
															file, _ := os.Create(fmt.Sprintf("%sconfig_%s.json", savePath, confName))
															file.Write(jconf)
															i++
														}
													}
												}
											} else {
												confName := fmt.Sprintf("uncles_%d_scaling_%f_maxdepth_%d_selfish_%d",u,p,md,0)
												conf = config{r, t, m, u, p, md, 1.0, nr, 0, 0, 0.0}
												jconf, err := json.Marshal(conf)
												if err == nil {
													file, _ := os.Create(fmt.Sprintf("%sconfig_%s.json", savePath, confName))
													file.Write(jconf)
													i++
												}
											}
										}
									}
								} else {
									for _, nr := range nephewReward {
										for _, s := range selfishMiners {
											if s > 0 {
												for _, sd := range selfishDelay {
													for _, sp := range selfishPower {
														confName := fmt.Sprintf("uncles_%d_scaling_%f_uncledivisor_%f_selfish_%d_sdelay_%d_spower_%f",u,p,ud,s,sd,sp)
														conf = config{r, t, m, u, p, 10000000000000, ud, nr, s, sd, sp}
														jconf, err := json.Marshal(conf)
														if err == nil {
															file, _ := os.Create(fmt.Sprintf("%sconfig_%s.json", savePath, confName))
															file.Write(jconf)
															i++
														}
													}
												}
											} else {
												confName := fmt.Sprintf("uncles_%d_scaling_%f_unclediv_%f_selfish_%d",u,p,ud,0)
												conf = config{r, t, m, u, p, 10000000000000, ud, nr, 0, 0, 0.0}
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
			}
		}
	}
}
