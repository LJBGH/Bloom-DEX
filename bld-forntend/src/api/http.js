import axios from 'axios'

export const userHttp = axios.create({
  baseURL: '/userapi',
  timeout: 15000,
})

export const walletHttp = axios.create({
  baseURL: '/walletapi',
  timeout: 15000,
})

