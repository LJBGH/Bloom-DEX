import { network } from "hardhat";
import { saveContractAddress } from "../utils.js";

async function main() {
  const { ethers } = await network.connect();
  const [deployer] = await ethers.getSigners();
  console.log("Deploying USDT with:", deployer.address);

  const USDT = await ethers.getContractFactory("USDT");
  const usdt = await USDT.deploy();
  await usdt.waitForDeployment();

  const address = await usdt.getAddress();
  console.log("USDT deployed to:", address);

  const networkName =
    (network as any).name || process.env.HARDHAT_NETWORK || "local";

  await saveContractAddress(networkName, "USDT", address);
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});

