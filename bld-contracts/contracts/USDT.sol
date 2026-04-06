// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract USDT is ERC20{
    uint8 private constant DECIMALS = 6;

    constructor() ERC20("Tether USD","USDT"){
        _mint(msg.sender, 100000000 * 10**DECIMALS);
    }

    function mint(address to, uint256 amount) external {
        _mint(to, amount);
    }

    function burn(address from, uint256 amount) external {
        _burn(from, amount);
    }

    function decimals() public pure override returns (uint8) {
        return DECIMALS;
    }
}