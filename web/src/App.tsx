import { useEffect, useState } from 'react'

export default function App() {
  const [health, setHealth] = useState('loading...')

  useEffect(() => {
    fetch('/api/health')
      .then((r) => r.text())
      .then(setHealth)
      .catch((e) => setHealth(`error: ${e}`))
  }, [])

  return (
    <main>
      <h1>Legacy Code Navigator</h1>
      <p>
        API health: <code>{health}</code>
      </p>
    </main>
  )
}
