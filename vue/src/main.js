import { createApp } from 'vue'
import { createRouter, createWebHashHistory } from 'vue-router'
import App from './App.vue'
import { store } from './store'
import './styles.css'

const Home = () => import('./views/Home.vue')
const Login = () => import('./views/Login.vue')
const Register = () => import('./views/Register.vue')
const ProblemDetail = () => import('./views/ProblemDetail.vue')
const Leaderboard = () => import('./views/Leaderboard.vue')
const StudyPlan = () => import('./views/StudyPlan.vue')
const Profile = () => import('./views/Profile.vue')
const MySubmissions = () => import('./views/MySubmissions.vue')
const SubmissionDetail = () => import('./views/SubmissionDetail.vue')
const AdminProblems = () => import('./views/admin/AdminProblems.vue')
const AdminProblemEdit = () => import('./views/admin/AdminProblemEdit.vue')
const AdminTags = () => import('./views/admin/AdminTags.vue')

const routes = [
  { path: '/', name: 'home', component: Home, meta: { title: 'Gojo OJ | 题库' } },
  { path: '/login', name: 'login', component: Login, meta: { title: 'Gojo OJ | 登录' } },
  { path: '/register', name: 'register', component: Register, meta: { title: 'Gojo OJ | 注册' } },
  { path: '/problems/:id', name: 'problem-detail', component: ProblemDetail, meta: { title: 'Gojo OJ | 题目详情' } },
  { path: '/leaderboard', name: 'leaderboard', component: Leaderboard, meta: { title: 'Gojo OJ | 排行榜' } },
  { path: '/study-plan', name: 'study-plan', component: StudyPlan, meta: { title: 'Gojo OJ | AI 训练计划', auth: true } },
  { path: '/profile', name: 'profile', component: Profile, meta: { title: 'Gojo OJ | 个人中心', auth: true } },
  { path: '/my-submissions', name: 'my-submissions', component: MySubmissions, meta: { title: 'Gojo OJ | 我的提交', auth: true } },
  { path: '/submissions/:id', name: 'submission-detail', component: SubmissionDetail, meta: { title: 'Gojo OJ | 提交详情', auth: true } },
  { path: '/admin/problems', name: 'admin-problems', component: AdminProblems, meta: { title: 'Gojo OJ | 题目管理', auth: true, admin: true } },
  { path: '/admin/problems/new', name: 'admin-problem-new', component: AdminProblemEdit, meta: { title: 'Gojo OJ | 新建题目', auth: true, admin: true } },
  { path: '/admin/problems/:id/edit', name: 'admin-problem-edit', component: AdminProblemEdit, meta: { title: 'Gojo OJ | 编辑题目', auth: true, admin: true } },
  { path: '/admin/tags', name: 'admin-tags', component: AdminTags, meta: { title: 'Gojo OJ | 标签管理', auth: true, admin: true } },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
  scrollBehavior() {
    return { top: 0, left: 0 }
  },
})

router.beforeEach((to) => {
  document.title = to.meta.title || 'Gojo OJ'

  if (to.meta.auth && !store.isLoggedIn) {
    return `/login?redirect=${encodeURIComponent(to.fullPath)}`
  }

  if (to.meta.admin && !store.isAdmin) {
    return '/'
  }

  return true
})

createApp(App).use(router).mount('#app')
