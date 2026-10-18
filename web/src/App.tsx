import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Alert, AppBar, Avatar, Badge, BottomNavigation, BottomNavigationAction, Box, Button, Card,
  CardActionArea, CardContent, Chip, CircularProgress, Container, Dialog, DialogActions,
  DialogContent, DialogTitle, Divider, Fab, FormControl, Grid, IconButton, InputLabel,
  MenuItem, Paper, Select, Snackbar, Stack, Switch, Tab, Tabs, TextField, ThemeProvider,
  Toolbar, Tooltip, Typography, createTheme, useMediaQuery
} from '@mui/material'
import {
  AddRounded, ArrowBackRounded, CalendarMonthOutlined, ChevronLeftRounded,
  ChevronRightRounded, CloseRounded, DeleteOutlineRounded, DownloadRounded, EditOutlined,
  GroupsOutlined, LocalFloristOutlined, LogoutRounded, PhotoCameraOutlined,
  PhotoLibraryOutlined, SettingsOutlined, YardOutlined, ArchiveOutlined, UnarchiveOutlined,
  CompareArrowsRounded, SearchRounded, CheckRounded
} from '@mui/icons-material'
import {
  api, json, setCsrf, thumb, original, localDateTime, withLocalOffset, displayDate,
  type Session, type User, type Member, type Plant, type PlantFields,
  type Species, type Entry, type EntryPage, type CalendarDay, type Photo
} from './api'

type Route = { page: 'home' | 'plant' | 'calendar' | 'family' | 'settings'; plantId?: number }
type Notify = (message: string, severity?: 'success' | 'error') => void
const emptyPlant: PlantFields = { name: '', species_id: 0, planted_on: '', location: '', notes: '', status: 'active' }

function routeFromHash(): Route {
  const path = location.hash.replace(/^#\/?/, '')
  if (path.startsWith('plant/')) {
    const id = Number(path.split('/')[1])
    if (Number.isSafeInteger(id) && id > 0) return { page: 'plant', plantId: id }
  }
  if (path === 'calendar' || path === 'family' || path === 'settings') return { page: path }
  return { page: 'home' }
}
function navigate(path: string) { location.hash = path }
function daysSince(date: string) {
  if (!date) return ''
  const days = Math.max(0, Math.floor((Date.now() - new Date(date + 'T00:00:00').getTime()) / 86400000))
  return '种植第 ' + (days + 1) + ' 天'
}
function imageUrl(photoId: number) { return photoId > 0 ? thumb(photoId) : '' }

export default function App() {
  const [route, setRoute] = useState<Route>(routeFromHash)
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [needsSetup, setNeedsSetup] = useState(false)
  const [dark, setDark] = useState(() => localStorage.getItem('garden-theme') === 'dark')
  const [notice, setNotice] = useState<{ message: string; severity: 'success' | 'error' } | null>(null)
  const theme = useMemo(() => createTheme({
    palette: {
      mode: dark ? 'dark' : 'light',
      primary: { main: dark ? '#95c59c' : '#38684e' },
      secondary: { main: '#b78754' },
      background: { default: dark ? '#161d19' : '#f7f6f0', paper: dark ? '#202923' : '#fffef9' },
    },
    shape: { borderRadius: 15 },
    typography: { fontFamily: 'Inter,-apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft Yahei",sans-serif', h4: { fontWeight: 750 }, h5: { fontWeight: 740 }, h6: { fontWeight: 700 } },
    components: {
      MuiButton: { defaultProps: { disableElevation: true }, styleOverrides: { root: { textTransform: 'none', borderRadius: 12 } } },
      MuiCard: { styleOverrides: { root: { boxShadow: '0 6px 26px rgba(26,58,35,.055)', borderRadius: 20 } } },
      MuiDialog: { styleOverrides: { paper: { borderRadius: 20 } } },
    },
  }), [dark])
  const mobile = useMediaQuery(theme.breakpoints.down('sm'))
  const notify: Notify = useCallback((message, severity = 'success') => setNotice({ message, severity }), [])
  useEffect(() => {
    const handler = () => setRoute(routeFromHash())
    window.addEventListener('hashchange', handler)
    return () => window.removeEventListener('hashchange', handler)
  }, [])
  useEffect(() => {
    let active = true
    api<{ needs_setup: boolean }>('/bootstrap')
      .then(async result => {
        if (!active) return
        setNeedsSetup(result.needs_setup)
        if (!result.needs_setup) {
          const session = await api<Session>('/me').catch(() => null)
          if (session && active) { setCsrf(session.csrf); setUser(session.user) }
        }
      })
      .catch(() => { if (active) notify('无法连接服务器，请检查部署状态', 'error') })
      .finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [notify])
  function toggleDark(checked: boolean) {
    setDark(checked)
    localStorage.setItem('garden-theme', checked ? 'dark' : 'light')
  }
  const navItems = [
    { value: 'home', label: '我的花园', icon: <YardOutlined /> },
    { value: 'calendar', label: '日历', icon: <CalendarMonthOutlined /> },
    { value: 'family', label: '家庭', icon: <GroupsOutlined /> },
    { value: 'settings', label: '设置', icon: <SettingsOutlined /> },
  ]
  return <ThemeProvider theme={theme}>
    <Box sx={{ minHeight: '100dvh', bgcolor: 'background.default', color: 'text.primary' }}>
      {loading ? <Stack sx={{ minHeight: '90vh' }} justifyContent="center" alignItems="center"><CircularProgress /></Stack>
        : !user ? <AuthScreen needsSetup={needsSetup} onSuccess={session => { setCsrf(session.csrf); setUser(session.user); setNeedsSetup(false); navigate('/') }} notify={notify} />
          : <>
            <AppBar position="sticky" elevation={0} color="transparent" sx={{ borderBottom: '1px solid', borderColor: 'divider', backdropFilter: 'blur(18px)', bgcolor: dark ? 'rgba(22,29,25,.92)' : 'rgba(247,246,240,.93)' }}>
              <Toolbar sx={{ minHeight: 66, gap: 1 }}>
                {route.page === 'plant' && <IconButton aria-label="返回花园" onClick={() => navigate('/')}><ArrowBackRounded /></IconButton>}
                <Box sx={{ bgcolor: 'primary.main', borderRadius: 2, color: dark ? '#152118' : '#fff', width: 38, height: 38, display: 'grid', placeItems: 'center' }}><YardOutlined /></Box>
                <Box sx={{ flexGrow: 1 }}>
                  <Typography fontWeight={800} letterSpacing={'.02em'}>花园日记</Typography>
                  {!mobile && <Typography variant="caption" color="text.secondary">Garden Diary · 一起见证每一次生长</Typography>}
                </Box>
                {!mobile && <Stack direction="row" spacing={.5}>
                  {navItems.map(item => <Button key={item.value} variant={(route.page === item.value || (item.value === 'home' && route.page === 'plant')) ? 'contained' : 'text'} color="primary" startIcon={item.icon} onClick={() => navigate('/' + (item.value === 'home' ? '' : item.value))}>{item.label}</Button>)}
                </Stack>}
                <Tooltip title={user.display_name}><Avatar sx={{ bgcolor: 'primary.light', color: '#133822', ml: 1, width: 34, height: 34 }}>{user.display_name.slice(0, 1)}</Avatar></Tooltip>
              </Toolbar>
            </AppBar>
            <Container maxWidth="lg" className="mobile-bottom-safe" sx={{ pt: { xs: 3, md: 5 }, pb: 8 }}>
              {route.page === 'home' && <Home key="home" notify={notify} />}
              {route.page === 'plant' && <PlantPage key={route.plantId} plantId={route.plantId!} notify={notify} />}
              {route.page === 'calendar' && <CalendarPage notify={notify} />}
              {route.page === 'family' && <FamilyPage currentUser={user} notify={notify} />}
              {route.page === 'settings' && <SettingsPage user={user} dark={dark} setDark={toggleDark} notify={notify} onLogout={() => { setUser(null); setCsrf(''); navigate('/') }} />}
            </Container>
            {mobile && <Paper elevation={6} sx={{ position: 'fixed', left: 0, right: 0, bottom: 0, zIndex: 30, borderTop: '1px solid', borderColor: 'divider', pb: 'env(safe-area-inset-bottom)' }}>
              <BottomNavigation value={route.page === 'plant' ? 'home' : route.page} onChange={(_, value) => navigate('/' + (value === 'home' ? '' : value))} showLabels>
                {navItems.map(item => <BottomNavigationAction key={item.value} value={item.value} label={item.label} icon={item.icon} />)}
              </BottomNavigation>
            </Paper>}
          </>}
      <Snackbar open={!!notice} autoHideDuration={4200} onClose={() => setNotice(null)} anchorOrigin={{ vertical: 'top', horizontal: 'center' }}>
        <Alert severity={notice?.severity || 'success'} variant="filled" onClose={() => setNotice(null)}>{notice?.message}</Alert>
      </Snackbar>
    </Box>
  </ThemeProvider>
}

function AuthScreen({ needsSetup, onSuccess, notify }: { needsSetup: boolean; onSuccess: (s: Session) => void; notify: Notify }) {
  const [username, setUsername] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [token, setToken] = useState('')
  const [busy, setBusy] = useState(false)
  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      const payload = needsSetup ? { username, display_name: displayName, password, setup_token: token } : { username, password }
      const session = await api<Session>(needsSetup ? '/setup' : '/login', json(payload))
      onSuccess(session)
    } catch (err) { notify((err as Error).message, 'error') }
    finally { setBusy(false) }
  }
  return <Container maxWidth="xs" sx={{ pt: { xs: 9, md: 14 } }}>
    <Stack alignItems="center" spacing={1} mb={5}>
      <Box sx={{ width: 76, height: 76, bgcolor: 'primary.main', borderRadius: 4, display: 'grid', placeItems: 'center', color: 'white' }}><LocalFloristOutlined sx={{ fontSize: 40 }} /></Box>
      <Typography variant="h4">花园日记</Typography>
      <Typography color="text.secondary">把每一个花开的日子，都好好保存。</Typography>
    </Stack>
    <Paper component="form" onSubmit={submit} sx={{ p: 3.5, borderRadius: 4 }}>
      <Typography variant="h6" mb={2}>{needsSetup ? '创建家庭花园' : '欢迎回到花园'}</Typography>
      {needsSetup && <Alert severity="info" sx={{ mb: 2 }}>首次部署请填写配置文件中的 GD_SETUP_TOKEN，并创建第一个管理员。</Alert>}
      <Stack spacing={2.2}>
        <TextField required label="用户名" value={username} onChange={e => setUsername(e.target.value)} autoComplete="username" />
        {needsSetup && <TextField required label="家庭昵称" value={displayName} onChange={e => setDisplayName(e.target.value)} />}
        <TextField required type="password" label="密码" value={password} onChange={e => setPassword(e.target.value)} autoComplete={needsSetup ? 'new-password' : 'current-password'} helperText={needsSetup ? '至少 10 个字符' : ''} />
        {needsSetup && <TextField required type="password" label="初始化口令" value={token} onChange={e => setToken(e.target.value)} />}
        <Button size="large" type="submit" variant="contained" disabled={busy}>{busy ? '处理中…' : needsSetup ? '创建花园' : '登录花园'}</Button>
      </Stack>
    </Paper>
  </Container>
}

function PlantDialog({ open, onClose, onSaved, plant, species, onSpecies, notify }: {
  open: boolean; onClose: () => void; onSaved: () => void; plant?: Plant | null;
  species: Species[]; onSpecies: () => void; notify: Notify
}) {
  const [fields, setFields] = useState<PlantFields>(emptyPlant)
  const [newSpecies, setNewSpecies] = useState('')
  const [saving, setSaving] = useState(false)
  useEffect(() => {
    setFields(plant ? { name: plant.name, species_id: plant.species_id, planted_on: plant.planted_on, location: plant.location, notes: plant.notes, status: plant.status } : { ...emptyPlant, planted_on: new Date().toLocaleDateString('en-CA') })
    setNewSpecies('')
  }, [plant, open])
  function change<K extends keyof PlantFields>(key: K, value: PlantFields[K]) { setFields(prev => ({ ...prev, [key]: value })) }
  async function createSpecies() {
    if (!newSpecies.trim()) return
    try {
      const s = await api<Species>('/species', json({ name: newSpecies.trim() }))
      change('species_id', s.id)
      setNewSpecies('')
      onSpecies()
      notify('已添加品种')
    } catch (err) { notify((err as Error).message, 'error') }
  }
  async function save() {
    if (!fields.name.trim()) { notify('请填写花卉名称', 'error'); return }
    setSaving(true)
    try {
      await api(plant ? '/plants/' + plant.id : '/plants', json(fields, plant ? 'PATCH' : 'POST'))
      onSaved()
      onClose()
      notify(plant ? '花卉档案已更新' : '新花卉已加入花园')
    } catch (err) { notify((err as Error).message, 'error') }
    finally { setSaving(false) }
  }
  return <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
    <DialogTitle>{plant ? '编辑花卉档案' : '种下一株新花'}</DialogTitle>
    <DialogContent><Stack spacing={2.5} sx={{ pt: 1 }}>
      <TextField autoFocus required label="给这株花起个名字" value={fields.name} onChange={e => change('name', e.target.value)} placeholder="例如：窗边的第一株月季" />
      <FormControl fullWidth><InputLabel>品种分组</InputLabel><Select label="品种分组" value={fields.species_id} onChange={e => change('species_id', Number(e.target.value))}>
        <MenuItem value={0}>未分类</MenuItem>{species.map(s => <MenuItem key={s.id} value={s.id}>{s.name}</MenuItem>)}
      </Select></FormControl>
      <Stack direction="row" spacing={1}><TextField size="small" label="新增品种" value={newSpecies} onChange={e => setNewSpecies(e.target.value)} fullWidth /><Button onClick={createSpecies} disabled={!newSpecies.trim()} variant="outlined">添加</Button></Stack>
      <TextField label="种植日期" type="date" slotProps={{ inputLabel: { shrink: true } }} value={fields.planted_on} onChange={e => change('planted_on', e.target.value)} />
      <TextField label="种植位置" placeholder="阳台 / 客厅 / 院子" value={fields.location} onChange={e => change('location', e.target.value)} />
      <TextField label="关于这株花" multiline rows={3} value={fields.notes} onChange={e => change('notes', e.target.value)} />
    </Stack></DialogContent>
    <DialogActions sx={{ px: 3, pb: 3 }}><Button onClick={onClose}>取消</Button><Button variant="contained" onClick={save} disabled={saving}>保存花卉</Button></DialogActions>
  </Dialog>
}

function Cover({ id, height = 210 }: { id: number; height?: number }) {
  return <Box sx={{ height, bgcolor: 'rgba(109,150,109,.13)', overflow: 'hidden', display: 'grid', placeItems: 'center', position: 'relative' }}>
    {id ? <Box component="img" src={imageUrl(id)} alt="花卉照片" loading="lazy" className="garden-photo" sx={{ width: '100%', height: '100%', objectFit: 'cover' }} />
      : <YardOutlined sx={{ color: 'primary.light', fontSize: 68, opacity: .65 }} />}
  </Box>
}

function Home({ notify }: { notify: Notify }) {
  const [plants, setPlants] = useState<Plant[]>([])
  const [species, setSpecies] = useState<Species[]>([])
  const [search, setSearch] = useState('')
  const [state, setState] = useState<'active' | 'archived'>('active')
  const [grouped, setGrouped] = useState(false)
  const [dialog, setDialog] = useState(false)
  const [busy, setBusy] = useState(true)
  const loadSpecies = useCallback(() => { api<Species[]>('/species').then(setSpecies).catch(err => notify(err.message, 'error')) }, [notify])
  const reload = useCallback(async () => {
    setBusy(true)
    try {
      const list = await api<Plant[]>('/plants?status=' + state + '&q=' + encodeURIComponent(search))
      setPlants(list)
    } catch (err) { notify((err as Error).message, 'error') }
    finally { setBusy(false) }
  }, [state, search, notify])
  useEffect(() => { reload() }, [reload])
  useEffect(() => { loadSpecies() }, [loadSpecies])
  const groups = useMemo(() => {
    const map = new Map<string, Plant[]>()
    for (const plant of plants) {
      const name = plant.species_name || '未分类'
      map.set(name, [...(map.get(name) || []), plant])
    }
    return Array.from(map.entries()).sort((a, b) => a[0].localeCompare(b[0], 'zh-CN'))
  }, [plants])
  function cards(list: Plant[]) {
    return <Grid container spacing={{ xs: 2, md: 3 }}>{list.map(p => <Grid item xs={6} sm={4} md={3} key={p.id}>
      <Card sx={{ height: '100%', overflow: 'hidden' }}><CardActionArea onClick={() => navigate('/plant/' + p.id)} sx={{ height: '100%', display: 'flex', flexDirection: 'column', alignItems: 'stretch' }}>
        <Cover id={p.latest_photo_id} height={190} />
        <CardContent sx={{ width: '100%', p: { xs: 1.5, md: 2 } }}>
          <Typography fontWeight={750} noWrap>{p.name}</Typography>
          <Typography variant="caption" color="text.secondary" display="block" noWrap>{p.species_name || '未分类'} {p.location ? ' · ' + p.location : ''}</Typography>
          <Typography variant="caption" color="primary.main" mt={1} display="block">{p.planted_on ? daysSince(p.planted_on) : p.photo_count + ' 张照片'}</Typography>
        </CardContent>
      </CardActionArea></Card>
    </Grid>)}</Grid>
  }
  return <Box className="garden-fade">
    <Stack direction={{ xs: 'column', sm: 'row' }} justifyContent="space-between" alignItems={{ xs: 'start', sm: 'center' }} spacing={2} mb={4}>
      <Box><Typography variant="h4" mb={.8}>我的花园 <Box component="span" sx={{ fontSize: 22 }}>🌿</Box></Typography>
        <Typography color="text.secondary">细心记录，静静等待每一次花开。</Typography></Box>
      <Button variant="contained" size="large" startIcon={<AddRounded />} onClick={() => setDialog(true)}>添加花卉</Button>
    </Stack>
    <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} mb={3} alignItems={{ xs: 'stretch', sm: 'center' }}>
      <TextField size="small" label="搜索花卉、品种或位置" value={search} onChange={e => setSearch(e.target.value)} sx={{ flex: 1 }} InputProps={{ startAdornment: <SearchRounded fontSize="small" sx={{ mr: 1, color: 'text.secondary' }} /> }} />
      <Stack direction="row" spacing={1}><Chip onClick={() => setState('active')} color={state === 'active' ? 'primary' : 'default'} label="正在生长" /><Chip onClick={() => setState('archived')} color={state === 'archived' ? 'primary' : 'default'} label="已归档" />
        <Chip onClick={() => setGrouped(!grouped)} variant="outlined" label={grouped ? '按单株' : '按品种'} /></Stack>
    </Stack>
    {busy ? <Stack alignItems="center" py={9}><CircularProgress /></Stack>
      : plants.length === 0 ? <Paper sx={{ py: 10, textAlign: 'center', borderRadius: 4 }}><LocalFloristOutlined sx={{ fontSize: 66, color: 'primary.light' }} /><Typography variant="h6" mt={2}>{state === 'archived' ? '还没有归档花卉' : '这片花园正等着第一株花'}</Typography><Typography color="text.secondary">添加花卉之后，就可以开始拍照记录了。</Typography></Paper>
        : grouped ? <Stack spacing={5}>{groups.map(([name, list]) => <Box key={name}><Typography variant="h6" mb={2}>{name} <Chip size="small" label={list.length + ' 株'} sx={{ ml: 1 }} /></Typography>{cards(list)}</Box>)}</Stack>
          : cards(plants)}
    <PlantDialog open={dialog} onClose={() => setDialog(false)} onSaved={reload} species={species} onSpecies={loadSpecies} notify={notify} />
  </Box>
}

function EntryDialog({ open, onClose, onSaved, plantId, edit, notify }: {
  open: boolean; onClose: () => void; onSaved: () => void; plantId: number; edit?: Entry | null; notify: Notify
}) {
  const [date, setDate] = useState(localDateTime())
  const [note, setNote] = useState('')
  const [files, setFiles] = useState<File[]>([])
  const [saving, setSaving] = useState(false)
  const camera = useRef<HTMLInputElement>(null)
  const gallery = useRef<HTMLInputElement>(null)
  useEffect(() => {
    setDate(localDateTime(edit?.observed_at))
    setNote(edit?.note || '')
    setFiles([])
  }, [edit, open])
  const previews = useMemo(() => files.map(file => ({ key: file.name + file.size, url: URL.createObjectURL(file), name: file.name })), [files])
  useEffect(() => () => previews.forEach(p => URL.revokeObjectURL(p.url)), [previews])
  function addFiles(incoming: FileList | null) {
    if (!incoming) return
    setFiles(current => [...current, ...Array.from(incoming!)].slice(0, 8))
  }
  async function save() {
    if (!date) { notify('请填写拍摄日期和时间', 'error'); return }
    if (!edit && files.length === 0) { notify('请至少选择一张照片', 'error'); return }
    setSaving(true)
    try {
      if (edit) {
        await api('/entries/' + edit.id, json({ observed_at: withLocalOffset(date), note }, 'PATCH'))
      } else {
        const body = new FormData()
        body.set('plant_id', String(plantId))
        body.set('observed_at', withLocalOffset(date))
        body.set('note', note)
        files.forEach(file => body.append('photos', file))
        await api('/entries', { method: 'POST', body })
      }
      onSaved()
      onClose()
      notify(edit ? '日记已更新' : '新的成长瞬间已记录')
    } catch (err) { notify((err as Error).message, 'error') }
    finally { setSaving(false) }
  }
  return <Dialog open={open} onClose={saving ? undefined : onClose} maxWidth="sm" fullWidth>
    <DialogTitle>{edit ? '编辑成长记录' : '记录今天的成长'}</DialogTitle>
    <DialogContent><Stack spacing={2.5} sx={{ pt: 1 }}>
      <TextField label="拍摄日期与时间" type="datetime-local" value={date} onChange={e => setDate(e.target.value)} slotProps={{ inputLabel: { shrink: true } }} />
      {!edit && <>
        <input ref={camera} type="file" accept="image/*,.heic,.heif" capture="environment" hidden onChange={e => { addFiles(e.target.files); e.target.value = '' }} />
        <input ref={gallery} type="file" accept="image/*,.heic,.heif" multiple hidden onChange={e => { addFiles(e.target.files); e.target.value = '' }} />
        <Stack direction="row" spacing={1}><Button fullWidth variant="contained" startIcon={<PhotoCameraOutlined />} onClick={() => camera.current?.click()}>拍照</Button><Button fullWidth variant="outlined" startIcon={<PhotoLibraryOutlined />} onClick={() => gallery.current?.click()}>从相册选择</Button></Stack>
        {files.length > 0 && <><Typography variant="caption" color="text.secondary">已选择 {files.length}/8 张（单张不超过 25 MiB）</Typography><Grid container spacing={1}>{previews.map((p, i) => <Grid item xs={4} key={i}><Box sx={{ position: 'relative' }}><Box component="img" src={p.url} alt={p.name} sx={{ borderRadius: 2, aspectRatio: '1', objectFit: 'cover', width: '100%', bgcolor: 'action.hover' }} /><IconButton aria-label="移除照片" size="small" onClick={() => setFiles(f => f.filter((_, j) => i !== j))} sx={{ position: 'absolute', right: 3, top: 3, bgcolor: 'background.paper' }}><CloseRounded fontSize="small" /></IconButton></Box></Grid>)}</Grid></>}
      </>}
      <TextField label="写下这一刻的故事（选填）" multiline minRows={3} value={note} onChange={e => setNote(e.target.value)} placeholder="新长了两片叶子，今天阳光特别好……" />
      {edit && <Alert severity="info">编辑记录可修改日期和备注；要替换照片请新建一条记录。</Alert>}
    </Stack></DialogContent>
    <DialogActions sx={{ px: 3, pb: 3 }}><Button disabled={saving} onClick={onClose}>取消</Button><Button variant="contained" disabled={saving} onClick={save}>{saving ? '正在保存照片…' : '保存记录'}</Button></DialogActions>
  </Dialog>
}

function PhotoViewer({ photos, index, onClose }: { photos: Photo[]; index: number; onClose: () => void }) {
  const [selected, setSelected] = useState(index)
  useEffect(() => setSelected(index), [index])
  const photo = photos[selected]
  if (!photo) return null
  return <Dialog open fullScreen onClose={onClose} PaperProps={{ sx: { bgcolor: '#101410', color: 'white' } }}>
    <Stack direction="row" alignItems="center" sx={{ px: 2, height: 62 }}>
      <IconButton sx={{ color: 'white' }} aria-label="关闭照片" onClick={onClose}><CloseRounded /></IconButton>
      <Typography sx={{ flex: 1, textAlign: 'center' }}>{selected + 1} / {photos.length}</Typography>
      <Tooltip title="下载原图"><IconButton component="a" href={original(photo.id) + '?download=1'} sx={{ color: 'white' }} aria-label="下载原图"><DownloadRounded /></IconButton></Tooltip>
    </Stack>
    <Stack direction="row" flex={1} alignItems="center" minHeight={0}>
      <IconButton sx={{ color: 'white' }} disabled={selected <= 0} onClick={() => setSelected(x => x - 1)} aria-label="上一张"><ChevronLeftRounded /></IconButton>
      <Box sx={{ flex: 1, display: 'grid', placeItems: 'center', height: '100%', minWidth: 0 }}><Box component="img" src={original(photo.id)} alt={photo.name} sx={{ maxHeight: 'calc(100dvh - 120px)', maxWidth: '100%', objectFit: 'contain' }} /></Box>
      <IconButton sx={{ color: 'white' }} disabled={selected >= photos.length - 1} onClick={() => setSelected(x => x + 1)} aria-label="下一张"><ChevronRightRounded /></IconButton>
    </Stack>
    <Typography textAlign="center" sx={{ py: 1.5, fontSize: 12, opacity: .8 }}>{photo.name}</Typography>
  </Dialog>
}

function PlantPage({ plantId, notify }: { plantId: number; notify: Notify }) {
  const [plant, setPlant] = useState<Plant | null>(null)
  const [species, setSpecies] = useState<Species[]>([])
  const [entries, setEntries] = useState<Entry[]>([])
  const [hasMore, setHasMore] = useState(false)
  const [loading, setLoading] = useState(true)
  const [moreBusy, setMoreBusy] = useState(false)
  const [view, setView] = useState(0)
  const [plantDialog, setPlantDialog] = useState(false)
  const [entryOpen, setEntryOpen] = useState(false)
  const [editing, setEditing] = useState<Entry | null>(null)
  const [viewer, setViewer] = useState<number | null>(null)
  const [left, setLeft] = useState(-1)
  const [right, setRight] = useState(-1)
  const load = useCallback(async () => {
    try {
      const [p, page, sp] = await Promise.all([api<Plant>('/plants/' + plantId), api<EntryPage>('/entries?plant_id=' + plantId + '&limit=50'), api<Species[]>('/species')])
      setPlant(p); setEntries(page.items); setHasMore(page.has_more); setSpecies(sp)
    } catch (err) { notify((err as Error).message, 'error') }
    finally { setLoading(false) }
  }, [plantId, notify])
  useEffect(() => { setLoading(true); load() }, [load])
  async function more() {
    setMoreBusy(true)
    try {
      const p = await api<EntryPage>('/entries?plant_id=' + plantId + '&limit=50&offset=' + entries.length)
      setEntries(e => [...e, ...p.items]);setHasMore(p.has_more)
    } catch (err) { notify((err as Error).message, 'error') }
    finally { setMoreBusy(false) }
  }
  const allPhotos = entries.flatMap(entry => entry.photos)
  const shots = entries.flatMap(entry => entry.photos.map(photo => ({ photo, date: entry.observed_at })))
  function openPhoto(id: number) { const index = allPhotos.findIndex(p => p.id === id); if (index >= 0) setViewer(index) }
  async function archive() {
    if (!plant) return
    try {
      const fields: PlantFields = { name: plant.name, species_id: plant.species_id, planted_on: plant.planted_on, location: plant.location, notes: plant.notes, status: plant.status === 'active' ? 'archived' : 'active' }
      await api('/plants/' + plant.id, json(fields, 'PATCH'))
      await load()
      notify(fields.status === 'archived' ? '已归档花卉' : '已恢复花卉')
    } catch (err) { notify((err as Error).message, 'error') }
  }
  async function removePlant() {
    if (!plant || !window.confirm('确定永久删除「' + plant.name + '」及所有成长照片吗？此操作无法撤销。')) return
    try { await api('/plants/' + plantId, { method: 'DELETE' }); notify('花卉已删除'); navigate('/') }
    catch (err) { notify((err as Error).message, 'error') }
  }
  async function removeEntry(id: number) {
    if (!window.confirm('确定永久删除这篇日记以及其中所有照片吗？')) return
    try { await api('/entries/' + id, { method: 'DELETE' }); await load(); notify('日记已删除') }
    catch (err) { notify((err as Error).message, 'error') }
  }
  if (loading) return <Stack alignItems="center" py={8}><CircularProgress /></Stack>
  if (!plant) return <Alert severity="warning">花卉不存在或已删除。</Alert>
  return <Box className="garden-fade">
    <Stack direction={{ xs: 'column', md: 'row' }} spacing={3} mb={4}>
      <Paper sx={{ width: { xs: '100%', md: 250 }, overflow: 'hidden', flexShrink: 0 }}><Cover id={plant.latest_photo_id} height={235} /></Paper>
      <Stack flex={1} justifyContent="center" spacing={1}>
        <Stack direction="row" alignItems="center" spacing={1}><Typography variant="h4">{plant.name}</Typography>{plant.status === 'archived' && <Chip label="已归档" size="small" />}</Stack>
        <Typography color="text.secondary">{plant.species_name || '未分类'}{plant.location ? ' · ' + plant.location : ''}</Typography>
        <Typography color="primary.main">{daysSince(plant.planted_on)} {plant.photo_count ? ' · ' + plant.photo_count + ' 张成长照片' : ''}</Typography>
        {plant.notes && <Typography sx={{ whiteSpace: 'pre-wrap', pt: 1 }}>{plant.notes}</Typography>}
        <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap sx={{ pt: 2 }}>
          <Button variant="contained" startIcon={<PhotoCameraOutlined />} onClick={() => { setEditing(null); setEntryOpen(true) }}>记录成长</Button>
          <Button variant="outlined" startIcon={<EditOutlined />} onClick={() => setPlantDialog(true)}>编辑</Button>
          <Tooltip title={plant.status === 'active' ? '归档花卉' : '恢复花卉'}><IconButton onClick={archive}>{plant.status === 'active' ? <ArchiveOutlined /> : <UnarchiveOutlined />}</IconButton></Tooltip>
          <Tooltip title="永久删除"><IconButton color="error" onClick={removePlant}><DeleteOutlineRounded /></IconButton></Tooltip>
        </Stack>
      </Stack>
    </Stack>
    <Paper sx={{ mb: 3, px: 1 }}><Tabs variant="scrollable" scrollButtons="auto" value={view} onChange={(_, next) => setView(next)}>
      <Tab label="成长时间线" icon={<CalendarMonthOutlined />} iconPosition="start" /><Tab label="照片" icon={<PhotoLibraryOutlined />} iconPosition="start" /><Tab label="成长对比" icon={<CompareArrowsRounded />} iconPosition="start" />
    </Tabs></Paper>
    {view === 0 && <Stack spacing={3} sx={{ maxWidth: 850, mx: 'auto' }}>
      {entries.length === 0 && <Paper sx={{ py: 7, textAlign: 'center' }}><YardOutlined sx={{ fontSize: 60, color: 'primary.light' }} /><Typography variant="h6">还没有成长记录</Typography><Typography color="text.secondary">拍下第一张照片，故事从这里开始。</Typography></Paper>}
      {entries.map(entry => <Box key={entry.id} sx={{ borderLeft: '3px solid', borderColor: 'primary.light', pl: 2.5, position: 'relative' }}>
        <Box sx={{ position: 'absolute', width: 13, height: 13, borderRadius: '50%', bgcolor: 'primary.main', left: -8, top: 19 }} />
        <Card><CardContent sx={{ pb: 2 }}>
          <Stack direction="row" alignItems="center" spacing={1} mb={1.5}>
            <Box flex={1}><Typography fontWeight={750}>{displayDate(entry.observed_at)}</Typography><Typography variant="caption" color="text.secondary">记录者：{entry.author}</Typography></Box>
            <IconButton aria-label="编辑日记" size="small" onClick={() => { setEditing(entry); setEntryOpen(true) }}><EditOutlined fontSize="small" /></IconButton>
            <IconButton aria-label="删除日记" size="small" color="error" onClick={() => removeEntry(entry.id)}><DeleteOutlineRounded fontSize="small" /></IconButton>
          </Stack>
          {entry.note && <Typography sx={{ whiteSpace: 'pre-wrap', mb: 2 }}>{entry.note}</Typography>}
          <Grid container spacing={1}>{entry.photos.map(photo => <Grid item xs={entry.photos.length === 1 ? 12 : 6} sm={entry.photos.length === 1 ? 8 : 4} key={photo.id}>
            <Box component="button" onClick={() => openPhoto(photo.id)} sx={{ border: 0, p: 0, cursor: 'pointer', width: '100%', bgcolor: 'action.hover', borderRadius: 2, overflow: 'hidden' }}>
              <Box component="img" loading="lazy" src={thumb(photo.id)} alt={photo.name} sx={{ width: '100%', aspectRatio: '1.2', objectFit: 'cover' }} />
            </Box>
          </Grid>)}</Grid>
        </CardContent></Card>
      </Box>)}
    </Stack>}
    {view === 1 && <Grid container spacing={1}>{shots.map(({ photo, date }) => <Grid item xs={4} sm={3} md={2} key={photo.id}>
      <Box component="button" onClick={() => openPhoto(photo.id)} sx={{ width: '100%', p: 0, border: 0, overflow: 'hidden', borderRadius: 1.5, cursor: 'pointer', bgcolor: 'action.hover' }}>
        <Box component="img" loading="lazy" src={thumb(photo.id)} alt={displayDate(date)} sx={{ width: '100%', aspectRatio: '1', objectFit: 'cover' }} />
      </Box>
    </Grid>)}</Grid>}
    {view === 2 && <Paper sx={{ p: { xs: 2, md: 3 } }}>
      <Typography variant="h6" mb={1}>看看它长大了多少</Typography>
      <Typography color="text.secondary" mb={2}>选择两张不同日期的照片，直观比较成长变化。</Typography>
      {shots.length < 2 ? <Alert severity="info">至少需要两张成长照片才可以对比。</Alert> : <>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
          {[left, right].map((value, i) => <FormControl fullWidth key={i}><InputLabel>{i === 0 ? '较早的照片' : '较新的照片'}</InputLabel><Select value={value < 0 ? (i === 0 ? shots.length - 1 : 0) : value} label={i === 0 ? '较早的照片' : '较新的照片'} onChange={e => i === 0 ? setLeft(Number(e.target.value)) : setRight(Number(e.target.value))}>
            {shots.map((s, j) => <MenuItem key={s.photo.id} value={j}>{displayDate(s.date)} · {s.photo.name}</MenuItem>)}
          </Select></FormControl>)}
        </Stack>
        <Grid container spacing={2} mt={.5}>{[left < 0 ? shots.length - 1 : left, right < 0 ? 0 : right].map((j, i) => <Grid item xs={6} key={i}>
          <Box component="img" src={original(shots[j]?.photo.id || shots[0].photo.id)} alt={i === 0 ? '较早照片' : '较新照片'} sx={{ width: '100%', objectFit: 'contain', maxHeight: 460, borderRadius: 2, bgcolor: 'action.hover' }} />
          <Typography color="text.secondary" variant="caption">{displayDate(shots[j]?.date || shots[0].date)}</Typography>
        </Grid>)}</Grid>
      </>}
    </Paper>}
    {hasMore && <Stack alignItems="center" mt={3}><Button variant="outlined" disabled={moreBusy} onClick={more}>{moreBusy ? '加载中…' : '加载更早记录'}</Button></Stack>}
    <PlantDialog open={plantDialog} plant={plant} species={species} onSpecies={() => api<Species[]>('/species').then(setSpecies)} onClose={() => setPlantDialog(false)} onSaved={load} notify={notify} />
    <EntryDialog open={entryOpen} edit={editing} plantId={plantId} onClose={() => setEntryOpen(false)} onSaved={load} notify={notify} />
    {viewer !== null && <PhotoViewer photos={allPhotos} index={viewer} onClose={() => setViewer(null)} />}
  </Box>
}

type DayItem = { entry_id: number; plant_id: number; plant_name: string; photo_id: number; observed_at: string; note: string }
function CalendarPage({ notify }: { notify: Notify }) {
  const [month, setMonth] = useState(() => new Date(new Date().getFullYear(), new Date().getMonth(), 1))
  const [days, setDays] = useState<CalendarDay[]>([])
  const [selected, setSelected] = useState('')
  const [items, setItems] = useState<DayItem[]>([])
  useEffect(() => { api<CalendarDay[]>('/calendar').then(setDays).catch(e => notify(e.message, 'error')) }, [notify])
  useEffect(() => {
    if (!selected) return
    api<DayItem[]>('/calendar/day?date=' + selected).then(setItems).catch(e => notify(e.message, 'error'))
  }, [selected, notify])
  const counts = useMemo(() => new Map(days.map(d => [d.day, d.count])), [days])
  const leading = (month.getDay() + 6) % 7
  const total = new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate()
  const cells = Array.from({ length: leading + total }, (_, i) => i < leading ? 0 : i - leading + 1)
  function dayKey(day: number) { return month.getFullYear() + '-' + String(month.getMonth() + 1).padStart(2, '0') + '-' + String(day).padStart(2, '0') }
  function offset(n: number) { setMonth(m => new Date(m.getFullYear(), m.getMonth() + n, 1)) }
  return <Box className="garden-fade"><Typography variant="h4" mb={.8}>成长日历</Typography><Typography color="text.secondary" mb={4}>记下花开花落的每一天。</Typography>
    <Paper sx={{ p: { xs: 1.5, sm: 3 }, maxWidth: 820, mx: 'auto' }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" mb={2}>
        <IconButton onClick={() => offset(-1)} aria-label="上个月"><ChevronLeftRounded /></IconButton>
        <Typography variant="h6">{month.getFullYear()} 年 {month.getMonth() + 1} 月</Typography>
        <IconButton onClick={() => offset(1)} aria-label="下个月"><ChevronRightRounded /></IconButton>
      </Stack>
      <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(7, 1fr)', gap: .8 }}>
        {['一', '二', '三', '四', '五', '六', '日'].map(d => <Typography key={d} textAlign="center" color="text.secondary" fontWeight={650} py={1}>{d}</Typography>)}
        {cells.map((d, i) => <Box key={i} component={d ? 'button' : 'div'} disabled={!d} onClick={d ? () => setSelected(dayKey(d)) : undefined}
          sx={{ height: { xs: 49, sm: 73 }, border: 0, borderRadius: 2, position: 'relative', cursor: d ? 'pointer' : 'default', bgcolor: d && selected === dayKey(d) ? 'primary.main' : 'transparent', color: d && selected === dayKey(d) ? 'white' : 'text.primary', '&:hover': d ? { bgcolor: selected === dayKey(d) ? 'primary.main' : 'action.hover' } : {} }}>
          {!!d && <><Typography fontWeight={selected === dayKey(d) ? 800 : 500}>{d}</Typography>{!!counts.get(dayKey(d)) && <Box sx={{ position: 'absolute', bottom: 6, left: '50%', transform: 'translateX(-50%)', width: 6, height: 6, borderRadius: '50%', bgcolor: selected === dayKey(d) ? 'white' : 'primary.main' }} />}</>}
        </Box>)}
      </Box>
      <Divider sx={{ my: 2 }} />
      {selected ? <><Typography fontWeight={720} mb={2}>{selected} · {counts.get(selected) || 0} 条记录</Typography>
        <Stack spacing={1.2}>{items.length === 0 ? <Typography color="text.secondary">这天还没有留下照片。</Typography>
          : items.map((item, i) => <Card key={i} variant="outlined"><CardActionArea onClick={() => navigate('/plant/' + item.plant_id)}><Stack direction="row" spacing={2} alignItems="center" p={1}>
            <Box sx={{ width: 75, height: 75, flexShrink: 0 }}><Cover id={item.photo_id} height={75} /></Box>
            <Box><Typography fontWeight={700}>{item.plant_name}</Typography><Typography variant="body2" color="text.secondary" noWrap>{item.note || displayDate(item.observed_at)}</Typography></Box>
          </Stack></CardActionArea></Card>)}</Stack>
      </> : <Typography color="text.secondary" textAlign="center">选择带有绿色圆点的日期，查看当天的花园日记。</Typography>}
    </Paper>
  </Box>
}

function FamilyPage({ currentUser, notify }: { currentUser: User; notify: Notify }) {
  const [members, setMembers] = useState<Member[]>([])
  const [open, setOpen] = useState(false)
  const [username, setUsername] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<'member' | 'admin'>('member')
  const reload = useCallback(() => { api<Member[]>('/users').then(setMembers).catch(e => notify(e.message, 'error')) }, [notify])
  useEffect(() => { reload() }, [reload])
  async function add() {
    try {
      await api('/users', json({ username, display_name: name, password, role }))
      setOpen(false);setUsername('');setName('');setPassword('');reload();notify('家庭成员已添加')
    } catch (err) { notify((err as Error).message, 'error') }
  }
  async function toggle(member: Member) {
    const action = member.disabled ? '启用' : '禁用'
    if (!window.confirm('确定' + action + '「' + member.display_name + '」的账号吗？')) return
    try { await api('/users/' + member.id, json({ disabled: !member.disabled }, 'PATCH')); reload(); notify('账号已' + action) }
    catch (err) { notify((err as Error).message, 'error') }
  }
  return <Box className="garden-fade"><Stack direction="row" alignItems="center" justifyContent="space-between" mb={1}><Typography variant="h4">家庭成员</Typography>{currentUser.role === 'admin' && <Button variant="contained" startIcon={<AddRounded />} onClick={() => setOpen(true)}>邀请家人</Button>}</Stack>
    <Typography color="text.secondary" mb={4}>所有家庭成员共同管理同一片花园，记录者会显示在每一篇日记上。</Typography>
    <Stack spacing={1.5}>{members.map(m => <Paper key={m.id} sx={{ p: 2.2 }}><Stack direction="row" alignItems="center" spacing={2}>
      <Avatar sx={{ bgcolor: 'primary.light', color: '#153f26' }}>{m.display_name[0]}</Avatar>
      <Box flex={1}><Typography fontWeight={720}>{m.display_name} {m.id === currentUser.id && '（我）'}</Typography><Typography variant="body2" color="text.secondary">@{m.username} · {m.role === 'admin' ? '管理员' : '家庭成员'}</Typography></Box>
      {m.disabled && <Chip size="small" color="warning" label="已禁用" />}
      {currentUser.role === 'admin' && m.id !== currentUser.id && <Button color={m.disabled ? 'primary' : 'error'} variant="outlined" size="small" onClick={() => toggle(m)}>{m.disabled ? '启用' : '禁用'}</Button>}
    </Stack></Paper>)}</Stack>
    <Dialog open={open} onClose={() => setOpen(false)} fullWidth maxWidth="xs"><DialogTitle>添加家庭成员</DialogTitle><DialogContent><Stack spacing={2} sx={{ pt: 1 }}>
      <TextField required label="登录用户名" value={username} onChange={e => setUsername(e.target.value)} helperText="3–32 位英文字母、数字、_ 或 -" />
      <TextField required label="显示昵称" value={name} onChange={e => setName(e.target.value)} />
      <TextField required type="password" label="初始密码" value={password} onChange={e => setPassword(e.target.value)} helperText="至少 10 个字符；请通过私密渠道告知家人。" />
      <FormControl><InputLabel>角色</InputLabel><Select label="角色" value={role} onChange={e => setRole(e.target.value as 'member' | 'admin')}><MenuItem value="member">家庭成员</MenuItem><MenuItem value="admin">管理员</MenuItem></Select></FormControl>
    </Stack></DialogContent><DialogActions><Button onClick={() => setOpen(false)}>取消</Button><Button variant="contained" onClick={add}>创建成员</Button></DialogActions></Dialog>
  </Box>
}

function SettingsPage({ user, dark, setDark, notify, onLogout }: { user: User; dark: boolean; setDark: (v: boolean) => void; notify: Notify; onLogout: () => void }) {
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [busy, setBusy] = useState(false)
  async function changePassword() {
    setBusy(true)
    try {
      await api('/password', json({ old_password: oldPassword, new_password: newPassword }))
      setOldPassword('');setNewPassword('');notify('密码已更改')
    } catch (err) { notify((err as Error).message, 'error') }
    finally { setBusy(false) }
  }
  async function logout() {
    try { await api('/logout', json({})) } catch { /* the browser session will still be cleared from the UI */ }
    onLogout()
  }
  return <Box className="garden-fade" sx={{ maxWidth: 700 }}><Typography variant="h4" mb={4}>设置</Typography>
    <Stack spacing={3}>
      <Paper sx={{ p: 3 }}><Typography variant="h6" mb={2}>账号</Typography><Typography mb={.5}>{user.display_name} <Chip size="small" label={user.role === 'admin' ? '管理员' : '家庭成员'} /></Typography><Typography color="text.secondary">@{user.username}</Typography><Divider sx={{ my: 2 }} />
        <Typography fontWeight={650} mb={1}>修改密码</Typography>
        <Stack spacing={1.5}><TextField type="password" label="当前密码" value={oldPassword} onChange={e => setOldPassword(e.target.value)} /><TextField type="password" label="新密码（10–72 字节）" value={newPassword} onChange={e => setNewPassword(e.target.value)} /><Button disabled={busy || !oldPassword || !newPassword} variant="outlined" onClick={changePassword}>更新密码</Button></Stack>
      </Paper>
      <Paper sx={{ p: 3 }}><Typography variant="h6" mb={2}>外观</Typography><Stack direction="row" alignItems="center" justifyContent="space-between"><Box><Typography>深色模式</Typography><Typography variant="body2" color="text.secondary">保护夜间赏花的眼睛</Typography></Box><Switch checked={dark} onChange={e => setDark(e.target.checked)} /></Stack></Paper>
      {user.role === 'admin' && <Paper sx={{ p: 3 }}><Typography variant="h6" mb={1}>数据导出</Typography><Typography color="text.secondary" variant="body2" mb={2}>将所有花卉、日记、照片原图导出为 ZIP。服务器完整恢复请使用命令行备份。</Typography><Button startIcon={<DownloadRounded />} variant="outlined" href="/api/v1/export">导出家庭花园</Button></Paper>}
      <Paper sx={{ p: 3 }}><Typography variant="h6" mb={1}>关于</Typography><Typography variant="body2" color="text.secondary">花园日记 · 家庭共享花卉成长记录。照片仅保存在你的自托管服务器中。</Typography></Paper>
      <Button variant="text" color="error" startIcon={<LogoutRounded />} onClick={logout}>退出登录</Button>
    </Stack>
  </Box>
}
