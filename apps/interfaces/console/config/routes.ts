/** 常用页面保留独立入口，官方示例统一归档到 demos。组件路径从 src/pages 开始解析。 */
export default [
  {
    path: '/user',
    layout: false,
    routes: [
      {
        path: '/user/login',
        name: 'login',
        component: './user/login',
      },
      {
        path: '/user',
        redirect: '/user/login',
      },
      {
        component: '404',
        path: '/user/*',
      },
    ],
  },
  {
    path: '/welcome',
    name: 'welcome',
    icon: 'home',
    component: './Welcome',
  },
  {
    name: 'account',
    icon: 'user',
    path: '/account',
    routes: [
      {
        path: '/account',
        redirect: '/account/center',
      },
      {
        name: 'center',
        icon: 'user',
        path: '/account/center',
        component: './account/center',
      },
      {
        name: 'settings',
        icon: 'setting',
        path: '/account/settings',
        component: './account/settings',
      },
    ],
  },
  {
    path: '/chatbot',
    name: 'chatbot',
    icon: 'robot',
    component: './chatbot',
  },
  {
    path: '/agent-development',
    name: 'agent-development',
    icon: 'experiment',
    component: './agent-development',
  },
  {
    path: '/api-explorer',
    name: 'api',
    icon: 'api',
    component: './api-explorer',
  },
  {
    path: '/demos',
    name: 'demos',
    icon: 'appstore',
    routes: [
      {
        path: '/demos',
        redirect: '/demos/dashboard/analysis',
      },
      {
        path: '/demos/go-agent',
        redirect: '/agent-development',
        hideInMenu: true,
      },
      {
        path: '/demos/dashboard',
        name: 'dashboard',
        icon: 'dashboard',
        routes: [
          {
            path: '/demos/dashboard',
            redirect: '/demos/dashboard/analysis',
          },
          {
            name: 'analysis',
            icon: 'barChart',
            path: '/demos/dashboard/analysis',
            component: './demos/dashboard/analysis',
          },
          {
            name: 'monitor',
            icon: 'monitor',
            path: '/demos/dashboard/monitor',
            component: './demos/dashboard/monitor',
          },
          {
            name: 'workplace',
            icon: 'desktop',
            path: '/demos/dashboard/workplace',
            component: './demos/dashboard/workplace',
          },
        ],
      },
      {
        path: '/demos/form',
        icon: 'form',
        name: 'form',
        routes: [
          {
            path: '/demos/form',
            redirect: '/demos/form/basic-form',
          },
          {
            name: 'basic-form',
            icon: 'form',
            path: '/demos/form/basic-form',
            component: './demos/form/basic-form',
          },
          {
            name: 'step-form',
            icon: 'orderedList',
            path: '/demos/form/step-form',
            component: './demos/form/step-form',
          },
          {
            name: 'advanced-form',
            icon: 'profile',
            path: '/demos/form/advanced-form',
            component: './demos/form/advanced-form',
          },
        ],
      },
      {
        path: '/demos/list',
        icon: 'table',
        name: 'list',
        routes: [
          {
            path: '/demos/list/search',
            name: 'search-list',
            component: './demos/list/search',
            routes: [
              {
                path: '/demos/list/search',
                redirect: '/demos/list/search/articles',
              },
              {
                name: 'articles',
                icon: 'read',
                path: '/demos/list/search/articles',
                component: './demos/list/search/articles',
              },
              {
                name: 'projects',
                icon: 'project',
                path: '/demos/list/search/projects',
                component: './demos/list/search/projects',
              },
              {
                name: 'applications',
                icon: 'appstore',
                path: '/demos/list/search/applications',
                component: './demos/list/search/applications',
              },
            ],
          },
          {
            path: '/demos/list',
            redirect: '/demos/list/table-list',
          },
          {
            name: 'table-list',
            icon: 'table',
            path: '/demos/list/table-list',
            component: './demos/table-list',
          },
          {
            name: 'basic-list',
            icon: 'unorderedList',
            path: '/demos/list/basic-list',
            component: './demos/list/basic-list',
          },
          {
            name: 'card-list',
            icon: 'creditCard',
            path: '/demos/list/card-list',
            component: './demos/list/card-list',
          },
        ],
      },
      {
        path: '/demos/profile',
        name: 'profile',
        icon: 'profile',
        routes: [
          {
            path: '/demos/profile',
            redirect: '/demos/profile/basic',
          },
          {
            name: 'basic',
            icon: 'idcard',
            path: '/demos/profile/basic',
            component: './demos/profile/basic',
          },
          {
            name: 'advanced',
            icon: 'crown',
            path: '/demos/profile/advanced',
            component: './demos/profile/advanced',
          },
        ],
      },
      {
        name: 'result',
        icon: 'checkCircle',
        path: '/demos/result',
        routes: [
          {
            path: '/demos/result',
            redirect: '/demos/result/success',
          },
          {
            name: 'success',
            icon: 'checkCircle',
            path: '/demos/result/success',
            component: './demos/result/success',
          },
          {
            name: 'fail',
            icon: 'closeCircle',
            path: '/demos/result/fail',
            component: './demos/result/fail',
          },
        ],
      },
      {
        name: 'exception',
        icon: 'warning',
        path: '/demos/exception',
        routes: [
          {
            path: '/demos/exception',
            redirect: '/demos/exception/403',
          },
          {
            name: '403',
            icon: 'stop',
            path: '/demos/exception/403',
            component: './demos/exception/403',
          },
          {
            name: '404',
            icon: 'warning',
            path: '/demos/exception/404',
            component: './demos/exception/404',
          },
          {
            name: '500',
            icon: 'bug',
            path: '/demos/exception/500',
            component: './demos/exception/500',
          },
        ],
      },
      {
        path: '/demos/admin',
        name: 'admin',
        icon: 'crown',
        access: 'canAdmin',
        routes: [
          {
            path: '/demos/admin',
            redirect: '/demos/admin/sub-page',
          },
          {
            path: '/demos/admin/sub-page',
            name: 'sub-page',
            component: './demos/Admin',
          },
        ],
      },
      {
        path: '/demos/user',
        name: 'user',
        icon: 'userAdd',
        routes: [
          {
            path: '/demos/user',
            redirect: '/demos/user/register',
          },
          {
            name: 'register',
            icon: 'userAdd',
            path: '/demos/user/register',
            component: './demos/user/register',
          },
          {
            name: 'register-result',
            icon: 'checkCircle',
            path: '/demos/user/register-result',
            component: './demos/user/register-result',
          },
        ],
      },
    ],
  },
  {
    path: '/',
    redirect: '/welcome',
  },
  {
    component: '404',
    path: './*',
  },
];
