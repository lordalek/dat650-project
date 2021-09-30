Project 2: Uncles and Uncles Rewards

The goal of this project is to model uncles and uncles' rewards. In this project, you are expected to:

Model a blockchain that uses uncles and uncle rewards.
Model rewarding mechanism to reward uncle block creators.
Model selfish mining (only one attacker) in this blockchain.
Try to answer the following questions with your experiments:

1- How do uncles improve the fairness of the blockchain? For this, you should compare the outcome of miners with and without uncles.

2- What is the impact of uncles on selfish mining? Is selfish mining more profitable with uncles?

3- What does it mean in this model for the selfish mining attack to be profitable?

Answers:
1- The possibility to receive a block reward, being not included in the
main chain, reduces the urge to form larger mining groups (pools).
2- The reward an uncle receives plus the reward for including an uncle is less then the block reward. This should incentivize nodes to try and extend the main chain.
3- The reward for uncles is reduces over distance. This disincentivises keeping blocks secret.