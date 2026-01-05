import React, { useState } from 'react';

function LoginScreen({ onLogin, connected }) {
  const [username, setUsername] = useState('');

  const handleSubmit = (e) => {
    e.preventDefault();
    if (username.trim() && connected) {
      onLogin(username.trim());
    }
  };

  return (
    <div className="login-screen">
      <p>Enter your username to start playing!</p>
      
      <form onSubmit={handleSubmit}>
        <input
          type="text"
          placeholder="Enter your username"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          maxLength={20}
          required
        />
        <button type="submit" disabled={!connected || !username.trim()}>
          {connected ? 'Join Game' : 'Connecting...'}
        </button>
      </form>
      
      {!connected && (
        <p style={{ color: '#dc3545', marginTop: '10px' }}>
          Connecting to server...
        </p>
      )}
    </div>
  );
}

export default LoginScreen;