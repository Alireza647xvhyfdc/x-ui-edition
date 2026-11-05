import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, ConfigProvider, Form, Input, Layout, Modal, Popconfirm, Space, Switch, Table, Tag, message } from 'antd';
import type { TableColumnsType } from 'antd';
import { CopyOutlined, DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { useTheme } from '@/hooks/useTheme';
import { usePageTitle } from '@/hooks/usePageTitle';
import { setMessageInstance } from '@/utils/messageBus';
import AppSidebar from '@/layouts/AppSidebar';
import { dayToExpiryMs, fetchSellerInfo, getSeller, postSeller } from './sellerApi';

interface ApiKey {
  id: number;
  name: string;
  enabled: boolean;
  scope: string;
  expiresAt: number;
  createdAt: number;
  token?: string;
}

interface PasswordForm {
  newUsername: string;
  newPassword: string;
  oldPassword: string;
  twoFactorCode?: string;
}

interface KeyForm {
  name: string;
  expireDay?: string;
}

export default function SellerSecurityPage() {
  usePageTitle();
  const { t } = useTranslation();
  const { antdThemeConfig } = useTheme();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);
  const queryClient = useQueryClient();
  const [passForm] = Form.useForm<PasswordForm>();
  const [keyForm] = Form.useForm<KeyForm>();
  const [keyModal, setKeyModal] = useState(false);
  const [createdKey, setCreatedKey] = useState<string | null>(null);

  const infoQuery = useQuery({ queryKey: ['seller', 'me'], queryFn: fetchSellerInfo });
  const keysQuery = useQuery({
    queryKey: ['seller', 'apiKeys'],
    queryFn: () => getSeller<ApiKey[]>('/panel/api/setting/apiTokens'),
  });
  const username = infoQuery.data?.username ?? '';

  const passwordMutation = useMutation({
    mutationFn: async (values: PasswordForm) =>
      postSeller('/panel/api/setting/updateUser', {
        oldUsername: username,
        oldPassword: values.oldPassword,
        newUsername: values.newUsername,
        newPassword: values.newPassword,
        twoFactorCode: values.twoFactorCode ?? '',
      }),
    onSuccess: () => {
      message.success(t('pages.seller.passwordChanged'));
      passForm.resetFields();
    },
    onError: (err: Error) => message.error(err.message),
  });

  const createKeyMutation = useMutation({
    mutationFn: async (values: KeyForm) =>
      postSeller<ApiKey>('/panel/api/setting/apiTokens/create', {
        name: values.name,
        scope: 'admin',
        expiresAt: dayToExpiryMs(values.expireDay),
      }),
    onSuccess: (key) => {
      setKeyModal(false);
      setCreatedKey(key.token ?? null);
      keyForm.resetFields();
      queryClient.invalidateQueries({ queryKey: ['seller', 'apiKeys'] });
    },
    onError: (err: Error) => message.error(err.message),
  });

  const toggleKeyMutation = useMutation({
    mutationFn: async ({ key, enabled }: { key: ApiKey; enabled: boolean }) =>
      postSeller(`/panel/api/setting/apiTokens/setEnabled/${key.id}`, { enabled, expectedScope: key.scope }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['seller', 'apiKeys'] }),
    onError: (err: Error) => message.error(err.message),
  });

  const deleteKeyMutation = useMutation({
    mutationFn: async (key: ApiKey) =>
      postSeller(`/panel/api/setting/apiTokens/delete/${key.id}`, { expectedScope: key.scope }),
    onSuccess: () => {
      message.success(t('pages.seller.keyDeleted'));
      queryClient.invalidateQueries({ queryKey: ['seller', 'apiKeys'] });
    },
    onError: (err: Error) => message.error(err.message),
  });

  const columns: TableColumnsType<ApiKey> = [
    { title: t('pages.seller.keyName'), dataIndex: 'name', key: 'name' },
    {
      title: t('pages.seller.status'),
      key: 'status',
      render: (_, k) => (k.enabled ? <Tag color="green">{t('pages.seller.active')}</Tag> : <Tag>{t('pages.seller.off')}</Tag>),
    },
    {
      title: t('pages.seller.expire'),
      key: 'expires',
      render: (_, k) => (k.expiresAt ? new Date(k.expiresAt).toLocaleDateString() : t('pages.seller.never')),
    },
    {
      title: '',
      key: 'actions',
      render: (_, k) => (
        <Space>
          <Switch
            size="small"
            checked={k.enabled}
            onChange={(enabled) => toggleKeyMutation.mutate({ key: k, enabled })}
            aria-label={t('pages.seller.enable')}
          />
          <Popconfirm
            title={t('pages.seller.keyDeleteConfirm')}
            okText={t('confirm')}
            cancelText={t('cancel')}
            onConfirm={() => deleteKeyMutation.mutate(k)}
          >
            <Button size="small" danger icon={<DeleteOutlined />} aria-label={t('pages.seller.delete')} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Card size="small" title={t('pages.seller.passwordTitle')} style={{ marginBottom: 16 }}>
              <Form form={passForm} layout="vertical" onFinish={(values) => passwordMutation.mutate(values)}>
                <Form.Item name="newUsername" label={t('pages.seller.username')} initialValue={username} rules={[{ required: true }]}>
                  <Input autoComplete="username" />
                </Form.Item>
                <Form.Item name="oldPassword" label={t('pages.seller.currentPassword')} rules={[{ required: true }]}>
                  <Input.Password autoComplete="current-password" />
                </Form.Item>
                <Form.Item name="newPassword" label={t('pages.seller.newPassword')} rules={[{ required: true }]}>
                  <Input.Password autoComplete="new-password" />
                </Form.Item>
                <Form.Item name="twoFactorCode" label={t('pages.seller.twoFactor')} extra={t('pages.seller.twoFactorHint')}>
                  <Input autoComplete="one-time-code" />
                </Form.Item>
                <Button type="primary" htmlType="submit" loading={passwordMutation.isPending}>
                  {t('pages.seller.changePassword')}
                </Button>
              </Form>
            </Card>

            <Card
              size="small"
              title={t('pages.seller.keysTitle')}
              extra={
                <Button type="primary" icon={<PlusOutlined />} onClick={() => setKeyModal(true)}>
                  {t('pages.seller.newKey')}
                </Button>
              }
            >
              <p>{t('pages.seller.keysHint')}</p>
              <Table<ApiKey>
                rowKey="id"
                size="small"
                loading={keysQuery.isLoading}
                columns={columns}
                dataSource={keysQuery.data ?? []}
                pagination={false}
                locale={{ emptyText: t('pages.seller.noKeys') }}
              />
            </Card>

            <Modal
              open={keyModal}
              title={t('pages.seller.newKey')}
              okText={t('save')}
              cancelText={t('cancel')}
              confirmLoading={createKeyMutation.isPending}
              onCancel={() => setKeyModal(false)}
              onOk={() => keyForm.validateFields().then((values) => createKeyMutation.mutate(values))}
              destroyOnHidden
            >
              <Form form={keyForm} layout="vertical">
                <Form.Item name="name" label={t('pages.seller.keyName')} rules={[{ required: true }]}>
                  <Input autoComplete="off" maxLength={64} />
                </Form.Item>
                <Form.Item name="expireDay" label={t('pages.seller.expire')} extra={t('pages.seller.keyExpireHint')}>
                  <Input type="date" />
                </Form.Item>
              </Form>
            </Modal>

            <Modal
              open={!!createdKey}
              title={t('pages.seller.keyCreated')}
              footer={null}
              onCancel={() => setCreatedKey(null)}
              destroyOnHidden
            >
              <p>{t('pages.seller.keyShownOnce')}</p>
              <Input.TextArea readOnly autoSize value={createdKey ?? ''} />
              <Button
                style={{ marginTop: 12 }}
                icon={<CopyOutlined />}
                onClick={() => navigator.clipboard.writeText(createdKey ?? '')}
              >
                {t('pages.seller.copy')}
              </Button>
            </Modal>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
