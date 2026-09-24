import React, { createContext, useContext, useEffect, useState } from 'react'
import { api, clearStoredTokens, getStoredToken, parseJwtClaims } from '../services/api'
import type { UserClaims } from '../types/api'

interface AuthContextType {
  user: UserClaims | null
  isAuthenticated: boolean
  loading: boolean
  login: (u: string, p: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextType | null>(null)

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<UserClaims | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const token = getStoredToken()
    if (token) {
      const claims = parseJwtClaims(token)
      if (claims && claims.usr && claims.rol) {
        setUser(claims as UserClaims)
      } else {
        clearStoredTokens()
      }
    }
    setLoading(false)
  }, [])

  const login = async (u: string, p: string) => {
    const tokens = await api.login(u, p)
    const claims = parseJwtClaims(tokens.access_token)
    if (claims) {
      setUser(claims as UserClaims)
    }
  }

  const logout = () => {
    api.logout()
    setUser(null)
  }

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated: !!user,
        loading,
        login,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}
