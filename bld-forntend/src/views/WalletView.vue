<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { listNetworksApi, createWalletApi, getPrivateKeyApi } from '../api/wallet'

const router = useRouter()
const userId = Number(localStorage.getItem('bld_user_id') || 0)
if (!userId) router.replace('/login')

const loading = ref(false)
const message = ref('')

const networks = ref([])
const createNetworkId = ref(0)
const createResult = ref(null)

const pkNetworkId = ref(0)
const pkConfirm = ref(false)
const pkReveal = ref(false)
const pkData = ref(null)

function showMsg(text) {
  message.value = text
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
    else showMsg(data?.message || err?.message || '请求失败')
  } finally {
    loading.value = false
  }
}

async function loadNetworks() {
  const { data } = await listNetworksApi()
  networks.value = data.items || []
  if (!createNetworkId.value && networks.value.length) {
    createNetworkId.value = networks.value[0].id
  }
  if (!pkNetworkId.value && networks.value.length) {
    pkNetworkId.value = networks.value[0].id
  }
}

onMounted(() => {
  runAction(loadNetworks)
})

async function submitCreate() {
  if (!createNetworkId.value) {
    showMsg('请选择网络')
    return
  }
  await runAction(async () => {
    const { data } = await createWalletApi({
      user_id: userId,
      network_id: createNetworkId.value,
    })
    createResult.value = data
  }, '钱包已就绪')
}

function openPkConfirm() {
  if (!pkNetworkId.value) {
    showMsg('请选择网络')
    return
  }
  pkData.value = null
  pkConfirm.value = true
}

async function confirmPk() {
  pkConfirm.value = false
  await runAction(async () => {
    const { data } = await getPrivateKeyApi({
      user_id: userId,
      network_id: pkNetworkId.value,
    })
    pkData.value = data
    pkReveal.value = true
  })
}

function closePkReveal() {
  pkReveal.value = false
  pkData.value = null
}

function copyText(text) {
  if (!text) return
  navigator.clipboard?.writeText(text)
  showMsg('已复制到剪贴板')
}
</script>

<template>
  <div>
    <section class="center-wrap">
      <div class="panel-title">托管钱包</div>
      <div class="panel-subtitle">
        网络列表来自后端；创建钱包须指定 network_id。同加密类型多链会复用同一套密钥。
      </div>

      <div class="grid" style="margin-top: 8px;">
        <div class="card asset-card">
          <div class="asset-head">
            <h3>创建钱包</h3>
          </div>
          <p class="hint" style="margin: 0 0 10px; color: #b91c1c;">
            创建钱包后，将自动生成钱包地址与私钥，请妥善保管私钥。
          </p>
          <div class="field">
            <span class="label">选择网络</span>
            <select v-model.number="createNetworkId">
              <option v-for="n in networks" :key="n.id" :value="n.id">
                {{ n.name }}
              </option>
            </select>
          </div>
          <button class="btn primary" :disabled="loading || !networks.length" @click="submitCreate">
            创建 / 获取钱包
          </button>
          <div v-if="createResult" class="mono" style="margin-top: 12px;">
            <div><strong>wallet_id:</strong> {{ createResult.wallet_id }}</div>
            <div><strong>network:</strong> {{ createResult.network_symbol }} (#{{ createResult.network_id }})</div>
            <div><strong>crypto_type:</strong> {{ createResult.chain }}</div>
            <div><strong>address:</strong> {{ createResult.address }}</div>
          </div>
        </div>

        <div class="card asset-card">
          <div class="asset-head">
            <h3>查看私钥</h3>
          </div>
          <p class="hint" style="margin: 0 0 10px; color: #b91c1c;">
            私钥仅展示于本机弹窗，请勿截屏外传。泄露将导致资产被盗。
          </p>
          <div class="field">
            <span class="label">选择网络</span>
            <select v-model.number="pkNetworkId">
              <option v-for="n in networks" :key="n.id" :value="n.id">
                {{ n.name }}
              </option>
            </select>
          </div>
          <button
            class="btn"
            style="background: #b91c1c; border-color: #b91c1c; color: #fff;"
            :disabled="loading || !networks.length"
            @click="openPkConfirm"
          >
            查看私钥
          </button>
        </div>
      </div>

      <p v-if="message" class="msg">{{ message }}</p>
    </section>

    <!-- 二次确认 -->
    <div v-if="pkConfirm" class="modal-mask" @click.self="pkConfirm = false">
      <div class="modal">
        <div class="modal-header">
          <div class="modal-title">确认查看私钥</div>
          <button class="btn" @click="pkConfirm = false">×</button>
        </div>
        <div class="modal-body">
          <p class="error" style="margin: 0 0 8px;">
            即将显示该网络下托管钱包的明文私钥。请确保环境安全，且你已了解风险。
          </p>
        </div>
        <div class="modal-footer">
          <button class="btn" @click="pkConfirm = false">取消</button>
          <button class="btn primary" :disabled="loading" @click="confirmPk">确认</button>
        </div>
      </div>
    </div>

    <!-- 私钥展示 -->
    <div v-if="pkReveal && pkData" class="modal-mask" @click.self="closePkReveal">
      <div class="modal">
        <div class="modal-header">
          <div class="modal-title">私钥（请勿泄露）</div>
          <button class="btn" @click="closePkReveal">×</button>
        </div>
        <div class="modal-body">
          <div class="field">
            <span class="label">地址</span>
            <div class="addr-box" style="padding-right: 16px;">
              {{ pkData.address }}
            </div>
          </div>
          <div class="field">
            <span class="label">crypto_type</span>
            <div>{{ pkData.crypto_type }}</div>
          </div>
          <div class="field">
            <span class="label">private_key</span>
            <div class="addr-box" style="padding-right: 88px;">
              {{ pkData.private_key }}
              <button type="button" class="btn link copy" @click="copyText(pkData.private_key)">复制</button>
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn full" @click="closePkReveal">关闭并清除</button>
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
</style>
