import { userHttp } from './http'

export function registerApi(payload) {
  return userHttp.post('/v1/register', payload)
}

export function loginApi(payload) {
  return userHttp.post('/v1/login', payload)
}

