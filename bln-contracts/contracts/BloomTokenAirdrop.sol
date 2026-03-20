//SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";

contract BloomTokenAirdrop {
    IERC20 public bloomToken;
    uint256 public totalTokensWithdrawn;

    mapping (address => bool) public wasClaimed;
    uint256 public constant TOKENS_PER_CLAIM = 100 * 10**18;

    event TokensAirdropped(address beneficiary);

    // 构造函数
    constructor(address _bloomToken) {
        require(_bloomToken != address(0));
        bloomToken = IERC20(_bloomToken);
    }

    // 领取代币
    function withdrawTokens() public {
        require(msg.sender == tx.origin, "Require that message sender is tx-origin.");

        address beneficiary = msg.sender;

        require(!wasClaimed[beneficiary], "Already claimed!");
        wasClaimed[msg.sender] = true;

        bool status = bloomToken.transfer(beneficiary, TOKENS_PER_CLAIM);
        require(status, "Token transfer status is false.");

        totalTokensWithdrawn = totalTokensWithdrawn+TOKENS_PER_CLAIM;
        emit TokensAirdropped(beneficiary);
    }
}
