document.addEventListener('DOMContentLoaded', async () => {
    try {
        const resp = await fetch('/api/auth/me', { credentials: 'same-origin' });
        if (!resp.ok) {
            window.location.href = '/static/login.html';
            return;
        }
    } catch (err) {
        window.location.href = '/static/login.html';
    }
});

const changePwdForm = document.getElementById('changePwdForm');
const changePwdMessage = document.getElementById('changePwdMessage');

changePwdForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const oldPassword = document.getElementById('oldPassword').value;
    const newPassword = document.getElementById('newPassword').value;
    const confirmPassword = document.getElementById('confirmPassword').value;

    if (!oldPassword || !newPassword) {
        showMsg('请输入原密码和新密码', 'error');
        return;
    }
    if (newPassword.length < 6) {
        showMsg('新密码至少6位', 'error');
        return;
    }
    if (newPassword !== confirmPassword) {
        showMsg('两次输入的新密码不一致', 'error');
        return;
    }

    try {
        const resp = await fetch('/api/auth/change-password', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'same-origin',
            body: JSON.stringify({ old_password: oldPassword, new_password: newPassword })
        });
        const data = await resp.json();
        if (resp.ok) {
            showMsg('密码修改成功，请重新登录', 'success');
            setTimeout(() => { window.location.href = '/static/login.html'; }, 1500);
        } else {
            showMsg(data.error || '修改失败', 'error');
        }
    } catch (err) {
        showMsg('网络错误', 'error');
    }
});

function showMsg(text, type) {
    changePwdMessage.textContent = text;
    changePwdMessage.className = 'message ' + type;
    setTimeout(() => { changePwdMessage.className = 'message'; }, 3000);
}