import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import MarketsView from '../views/MarketsView.vue'
import TradeView from '../views/TradeView.vue'
import AssetsView from '../views/AssetsView.vue'
import WalletView from '../views/WalletView.vue'
import LoginView from '../views/LoginView.vue'
import RegisterView from '../views/RegisterView.vue'

const routes = [
  { path: '/', redirect: '/home' },
  { path: '/home', name: 'home', component: HomeView },
  { path: '/markets', name: 'markets', component: MarketsView },
  { path: '/trade', redirect: '/trade/spot' },
  { path: '/trade/spot', name: 'trade_spot', component: TradeView },
  { path: '/trade/perp', name: 'trade_perp', component: TradeView },
  { path: '/assets', name: 'assets', component: AssetsView },
  { path: '/wallet', name: 'wallet', component: WalletView },
  { path: '/login', name: 'login', component: LoginView },
  { path: '/register', name: 'register', component: RegisterView },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

export default router

