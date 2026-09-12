const API_BASE = '/api';
const createUserForm = document.getElementById('createUserForm');
const createMessage = document.getElementById('createMessage');

createUserForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const username = document.getElementById('newUsername').value.trim();
    const password = document.getElementById('newPassword').value;

    if (!username) {
        showMsg(createMessage, '请输入用户名', 'error');
        return;
    }
    if (username.length < 2) {
        showMsg(createMessage, '用户名至少2位', 'error');
        return;
    }
    if (password.length < 6) {
        showMsg(createMessage, '密码至少6位', 'error');
        return;
    }

    try {
        const resp = await fetch(API_BASE + '/admin/users', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'same-origin',
            body: JSON.stringify({ username, password })
        });
        const data = await resp.json();
        if (resp.ok) {
            showMsg(createMessage, '创建成功', 'success');
            document.getElementById('newUsername').value = '';
            document.getElementById('newPassword').value = '';
            loadUsers();
        } else {
            showMsg(createMessage, data.error || '创建失败', 'error');
        }
    } catch (err) {
        showMsg(createMessage, '网络错误', 'error');
    }
});

async function loadUsers() {
    const usersList = document.getElementById('usersList');
    usersList.innerHTML = '<p class="loading">加载中...</p>';
    try {
        const resp = await fetch(API_BASE + '/admin/users', { credentials: 'same-origin' });
        const data = await resp.json();
        if (!resp.ok) {
            usersList.innerHTML = '<p class="empty">' + escapeHtml(data.error || '加载失败') + '</p>';
            return;
        }
        const users = data.users || [];
        if (users.length === 0) {
            usersList.innerHTML = '<p class="empty">暂无子账号</p>';
            return;
        }
        usersList.innerHTML = users.map(u => `
            <div class="file-item">
                <div class="file-info">
                    <div class="file-name">${escapeHtml(u.username)}</div>
                    <div class="file-meta">ID: ${u.id}</div>
                </div>
                <div class="file-actions">
                    <button onclick="resetPassword('${escapeHtml(u.username)}')" class="btn btn-small">重置密码</button>
                    <button onclick="deleteUser(${u.id})" class="btn btn-delete">删除</button>
                </div>
            </div>
        `).join('');
    } catch (err) {
        usersList.innerHTML = '<p class="empty">加载失败</p>';
    }
}

async function resetPassword(username) {
    if (!confirm('确认将 ' + username + ' 的密码重置为 123456？')) return;
    try {
        const resp = await fetch(API_BASE + '/auth/reset-password', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'same-origin',
            body: JSON.stringify({ username })
        });
        const data = await resp.json();
        alert(data.message || data.error || '操作完成');
    } catch (err) {
        alert('网络错误');
    }
}

async function deleteUser(id) {
    if (!confirm('确认删除该账号？')) return;
    try {
        const resp = await fetch(API_BASE + '/admin/users/' + id, {
            method: 'DELETE',
            credentials: 'same-origin'
        });
        const data = await resp.json();
        if (resp.ok) {
            loadUsers();
        } else {
            alert(data.error || '删除失败');
        }
    } catch (err) {
        alert('网络错误');
    }
}

function showMsg(el, text, type) {
    el.textContent = text;
    el.className = 'message ' + type;
    setTimeout(() => { el.className = 'message'; }, 3000);
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

loadUsers();