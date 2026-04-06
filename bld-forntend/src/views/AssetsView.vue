<script setup>
import { computed, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  getDepositAddressApi,
  listAssetsApi,
  listNetworksApi,
  listTokensApi,
  sweepToHotApi,
  withdrawApi,
} from '../api/wallet'

const router = useRouter()
const userId = Number(localStorage.getItem('bld_user_id') || 0)
if (!userId) router.replace('/login')

const loading = ref(false)
const message = ref('')
const assets = ref([])

const hasAssets = computed(() => assets.value.length > 0)

// dialogs
const showDeposit = ref(false)
const showWithdraw = ref(false)
const showSweep = ref(false)

const depositState = reactive({
  networkId: 0,
  networks: [],
  tokens: [],
  symbol: '',
  chainLabel: '',
  address: '',
  decimals: '',
  contract: '',
})

const withdrawForm = reactive({
  symbol: 'ETH',
  chainLabel: 'localhost:8545 (本地EVM测试链)',
  dest_address: '',
  amount: '',
})

const sweepForm = reactive({
  symbol: 'ETH',
  amount: '0',
})

function showMsg(text) {
  message.value = text
}

function getAvail(symbol) {
  const row = assets.value.find((x) => x.symbol === symbol)
  return row?.available_balance || '0'
}

async function runAction(fn, okText) {
  loading.value = true
  message.value = ''
  try {
    await fn()
    if (okText) showMsg(okText)
  } catch (err) {
    const data = err?.response?.data
    if (typeof data === 'string' && data.trim()) showMsg(data)
    else showMsg(data?.message || err?.message || 'Request failed')
  } finally {
    loading.value = false
  }
}

async function refreshAssets() {
  await runAction(async () => {
    const { data } = await listAssetsApi(userId)
    assets.value = data.items || []
  })
}

async function loadNetworks() {
  const { data } = await listNetworksApi()
  depositState.networks = data.items || []
  if (!depositState.networkId && depositState.networks.length) {
    depositState.networkId = depositState.networks[0].id
  }
  const n = depositState.networks.find((x) => x.id === depositState.networkId)
  depositState.chainLabel = n ? n.name : ''
}

async function loadTokens(networkId) {
  const { data } = await listTokensApi(networkId)
  depositState.tokens = data.items || []
  if (!depositState.symbol && depositState.tokens.length) {
    depositState.symbol = depositState.tokens[0].symbol
  }
}

async function fetchDepositAddress() {
  depositState.address = ''
  depositState.decimals = ''
  depositState.contract = ''
  const { data } = await getDepositAddressApi({
    user_id: userId,
    symbol: depositState.symbol,
    network_id: depositState.networkId,
  })
  depositState.address = data.address
  depositState.decimals = String(data.decimals ?? '')
  depositState.contract = data.contract_address || ''
}

async function openDeposit() {
  showDeposit.value = true
  await runAction(async () => {
    await loadNetworks()
    if (!depositState.networkId) return
    await loadTokens(depositState.networkId)
    if (!depositState.symbol) return
    await fetchDepositAddress()
  })
}

function copyText(text) {
  if (!text) return
  navigator.clipboard?.writeText(text)
  showMsg('已复制到剪贴板')
}

function openWithdraw(symbol) {
  withdrawForm.symbol = symbol
  withdrawForm.dest_address = ''
  withdrawForm.amount = ''
  showWithdraw.value = true
}

function setWithdrawMax() {
  withdrawForm.amount = String(getAvail(withdrawForm.symbol))
}

async function submitWithdraw() {
  await runAction(async () => {
    await withdrawApi({
      user_id: userId,
      symbol: withdrawForm.symbol,
      dest_address: withdrawForm.dest_address,
      amount: withdrawForm.amount,
      chain: 'EVM',
    })
    showWithdraw.value = false
    await refreshAssets()
  }, '已提交提现')
}

function openSweep(symbol) {
  sweepForm.symbol = symbol
  sweepForm.amount = '0'
  showSweep.value = true
}

async function submitSweep() {
  await runAction(async () => {
    await sweepToHotApi({
      user_id: userId,
      symbol: sweepForm.symbol,
      amount: sweepForm.amount || '0',
      chain: 'EVM',
    })
    showSweep.value = false
    await refreshAssets()
  }, '已提交划转到热钱包')
}

refreshAssets()
</script>

<template>
  <div>
    <section class="center-wrap">
      <div class="panel-title">现货账户</div>
      <div class="panel-subtitle">用户可以在这里进行充值提现</div>

      <div class="card asset-card">
        <div class="asset-head">
          <h3>现货账户</h3>
          <div style="display:flex; gap:10px; align-items:center;">
            <button class="btn" :disabled="loading" @click="openDeposit">充值</button>
            <button class="btn primary" :disabled="loading" @click="refreshAssets">刷新</button>
          </div>
        </div>

        <div class="hint" style="margin-bottom:10px;">资产列表</div>

        <table class="table compact" v-if="hasAssets">
          <thead>
            <tr>
              <th style="width: 30%;">币种</th>
              <th style="width: 40%;">余额</th>
              <th style="width: 30%;">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in assets" :key="item.asset_id">
              <td>{{ item.symbol }}</td>
              <td>{{ item.available_balance }}</td>
              <td>
                <div class="op">
                  <button class="btn link" @click="openWithdraw(item.symbol)">提现</button>
                  <button class="btn link" @click="openSweep(item.symbol)">划转</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
        <p v-else class="hint">
          暂无资产记录（可先<router-link to="/wallet" class="nav-link" style="display:inline;">创建钱包</router-link>并充值）
        </p>
      </div>

      <p v-if="message" class="msg">{{ message }}</p>
    </section>

    <!-- Deposit Modal -->
    <div v-if="showDeposit" class="modal-mask" @click.self="showDeposit = false">
      <div class="modal">
        <div class="modal-header">
          <div class="modal-title">充值</div>
          <button class="btn" @click="showDeposit = false">×</button>
        </div>
        <div class="modal-body">
          <div class="field">
            <span class="label">网络</span>
            <select
              v-model.number="depositState.networkId"
              @change="
                runAction(async () => {
                  depositState.symbol = ''
                  await loadTokens(depositState.networkId)
                  const n = depositState.networks.find((x) => x.id === depositState.networkId)
                  depositState.chainLabel = n ? n.name : ''
                  if (depositState.symbol) await fetchDepositAddress()
                })
              "
            >
              <option v-for="n in depositState.networks" :key="n.id" :value="n.id">
                {{ n.name }}
              </option>
            </select>
          </div>
          <div class="field">
            <span class="label">代币</span>
            <select
              v-model="depositState.symbol"
              @change="
                runAction(async () => {
                  if (!depositState.symbol) return
                  await fetchDepositAddress()
                })
              "
            >
              <option v-for="t in depositState.tokens" :key="t.asset_id" :value="t.symbol">
                {{ t.symbol }}
              </option>
            </select>
          </div>
          <div class="field">
            <span class="label">充值地址</span>
            <div class="addr-box">
              {{ depositState.address || '加载中...' }}
              <button class="btn link copy" @click="copyText(depositState.address)">复制</button>
            </div>
          </div>
          <div class="hint" v-if="depositState.contract">
            USDT 合约：{{ depositState.contract }}
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn full" @click="showDeposit = false">关闭</button>
        </div>
      </div>
    </div>

    <!-- Withdraw Modal -->
    <div v-if="showWithdraw" class="modal-mask" @click.self="showWithdraw = false">
      <div class="modal">
        <div class="modal-header">
          <div class="modal-title">提现</div>
          <button class="btn" @click="showWithdraw = false">×</button>
        </div>
        <div class="modal-body">
          <div class="field">
            <span class="label">币种</span>
            <div>{{ withdrawForm.symbol }}</div>
          </div>
          <div class="field">
            <span class="label">网络</span>
            <select v-model="withdrawForm.chainLabel">
              <option>localhost:8545 (本地EVM测试链)</option>
            </select>
          </div>
          <div class="field">
            <span class="label">提现地址</span>
            <input v-model="withdrawForm.dest_address" placeholder="请输入提现地址" />
          </div>
          <div class="field">
            <span class="label">提现数量</span>
            <div class="input-row">
              <input v-model="withdrawForm.amount" placeholder="请输入提现数量" />
              <button class="max-btn" @click="setWithdrawMax">MAX</button>
            </div>
            <div class="hint" style="margin-top:6px;">
              可用余额：{{ getAvail(withdrawForm.symbol) }} {{ withdrawForm.symbol }}
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn" @click="showWithdraw = false">取消</button>
          <button class="btn primary" :disabled="loading" @click="submitWithdraw">确认提现</button>
        </div>
      </div>
    </div>

    <!-- Sweep Modal -->
    <div v-if="showSweep" class="modal-mask" @click.self="showSweep = false">
      <div class="modal">
        <div class="modal-header">
          <div class="modal-title">划转到热钱包</div>
          <button class="btn" @click="showSweep = false">×</button>
        </div>
        <div class="modal-body">
          <div class="field">
            <span class="label">币种</span>
            <div>{{ sweepForm.symbol }}</div>
          </div>
          <div class="field">
            <span class="label">数量（0=全部）</span>
            <input v-model="sweepForm.amount" placeholder="0 或 输入数量" />
            <div class="hint" style="margin-top:6px;">
              热钱包地址为后端配置的 HotWalletAddress
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn" @click="showSweep = false">取消</button>
          <button class="btn primary" :disabled="loading" @click="submitSweep">确认划转</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.asset-card { padding: 22px; border-radius: 8px; }
.asset-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--dex-border, #2a2e36);
  padding-bottom: 10px;
  margin-bottom: 14px;
}
.asset-head h3 {
  margin: 0;
  font-size: 18px;
  color: var(--dex-text, #eaecef);
}

.table.compact th,
.table.compact td {
  text-align: center;
  vertical-align: middle;
  padding: 10px 8px;
  font-size: 13px;
}
.op { display: flex; gap: 10px; justify-content: center; align-items: center; }

.modal-mask {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 999;
}
.modal {
  width: 560px;
  max-width: calc(100vw - 24px);
  background: var(--dex-bg-panel, #14161a);
  border: 1px solid var(--dex-border, #2a2e36);
  border-radius: 8px;
  overflow: hidden;
  color: var(--dex-text, #eaecef);
}
.modal-header {
  padding: 14px 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid var(--dex-border, #2a2e36);
}
.modal-header .btn { margin-top: 0; padding: 4px 10px; line-height: 1; }
.modal-title { font-size: 16px; font-weight: 600; color: var(--dex-text, #eaecef); }
.modal-body { padding: 16px; }
.field { margin-bottom: 12px; }
.field .label { display: block; font-size: 12px; color: var(--dex-text-secondary, #848e9c); margin-bottom: 6px; }
.addr-box {
  border: 1px solid var(--dex-border, #2a2e36);
  border-radius: 6px;
  padding: 10px;
  padding-right: 88px;
  background: var(--dex-bg-input, #12141a);
  color: var(--dex-text, #eaecef);
  font-family: ui-monospace, 'Cascadia Code', 'Courier New', monospace;
  font-size: 12px;
  word-break: break-all;
  position: relative;
}
.addr-box .copy {
  position: absolute;
  right: 10px;
  top: 50%;
  transform: translateY(-50%);
  font-family: Arial, Helvetica, sans-serif;
}
.modal-footer {
  padding: 12px 16px;
  border-top: 1px solid var(--dex-border, #2a2e36);
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.modal-footer .btn { width: 100%; margin-top: 0; }
.modal-footer .btn.full { grid-column: 1 / -1; }

.input-row {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 8px;
}
.max-btn {
  width: auto;
  padding: 0 10px;
  border-radius: 6px;
  border: 1px solid var(--dex-border-soft, #3d4454);
  background: var(--dex-bg-elevated, #1e2026);
  color: var(--dex-accent, #f0b90b);
  cursor: pointer;
  margin-top: 0;
  font-weight: 700;
}
</style>

