import axios from 'axios'

export const userHttp = axios.create({
  baseURL: '/api/userapi',
  timeout: 15000,
})

export const walletHttp = axios.create({
  baseURL: '/api/walletapi',
  timeout: 15000,
})

