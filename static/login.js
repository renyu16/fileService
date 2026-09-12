const loginForm = document.getElementById('loginForm');
const loginMessage = document.getElementById('loginMessage');

loginForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const username = document.getElementById('username').value.trim();
    const password = document.getElementById('password').value;

    if (!username || !password) {
        showLoginMessage('请输入用户名和密码', 'error');
        return;
    }

    try {
        const resp = await fetch('/api/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'same-origin',
            body: JSON.stringify({ username, password })
        });
        const data = await resp.json();
        if (resp.ok) {
            window.location.href = '/';
        } else {
            showLoginMessage(data.error || '登录失败', 'error');
        }
    } catch (err) {
        showLoginMessage('网络错误', 'error');
    }
});

function showLoginMessage(text, type) {
    loginMessage.textContent = text;
    loginMessage.className = 'message ' + type;
    setTimeout(() => { loginMessage.className = 'message'; }, 3000);
}