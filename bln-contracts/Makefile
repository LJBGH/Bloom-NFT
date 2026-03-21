all:
	@npx hardhat compile
	@npx hardhat run --network local scripts/deployment/deploy_token.ts
	@npx hardhat run --network local scripts/deployment/deploy_nft.ts
	@npx hardhat run --network local scripts/deployment/deploy_marketplace.ts
	@npx hardhat run --network local scripts/deployment/deploy_token_airdorp.ts