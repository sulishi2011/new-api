/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Button, Form, Modal, Space, Table, Tag } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../../helpers';

const EMPTY_PROFILE = {
  code: '',
  vendor_code: '',
  vendor_name: '',
  platform_type: '',
  discount_code: '',
  discount_label: '',
  discount_rate: undefined,
  status: 1,
  notes: '',
};

const VendorProfileModal = ({ visible, handleClose, refresh, t }) => {
  const formApiRef = useRef(null);
  const [loading, setLoading] = useState(false);
  const [profiles, setProfiles] = useState([]);
  const [editingProfile, setEditingProfile] = useState(null);

  const loadProfiles = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/vendor_profiles/?page_size=1000');
      const { success, message, data } = res?.data || {};
      if (success) {
        setProfiles(data?.items || []);
      } else {
        showError(message);
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (visible) {
      loadProfiles();
      setEditingProfile(null);
      setTimeout(() => formApiRef.current?.setValues(EMPTY_PROFILE), 0);
    }
  }, [visible]);

  const startEdit = (profile) => {
    setEditingProfile(profile);
    formApiRef.current?.setValues({
      ...EMPTY_PROFILE,
      ...profile,
      discount_rate:
        profile.discount_rate === null || profile.discount_rate === undefined
          ? undefined
          : Number(profile.discount_rate),
    });
  };

  const resetForm = () => {
    setEditingProfile(null);
    formApiRef.current?.setValues(EMPTY_PROFILE);
  };

  const submit = async (values) => {
    const discountRate =
      values.discount_rate === '' || values.discount_rate === undefined
        ? null
        : Number(values.discount_rate);
    const payload = {
      ...values,
      discount_rate: Number.isFinite(discountRate) ? discountRate : null,
      status: values.status || 1,
    };
    const res = editingProfile?.id
      ? await API.put('/api/vendor_profiles/', {
          ...payload,
          id: editingProfile.id,
        })
      : await API.post('/api/vendor_profiles/', payload);
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('保存成功'));
      resetForm();
      await loadProfiles();
      refresh?.();
    } else {
      showError(message);
    }
  };

  const deleteProfile = (profile) => {
    Modal.confirm({
      title: t('确定删除供应商配置？'),
      content: profile.code,
      onOk: async () => {
        const res = await API.delete(`/api/vendor_profiles/${profile.id}`);
        const { success, message } = res.data;
        if (success) {
          showSuccess(t('删除成功'));
          await loadProfiles();
          refresh?.();
        } else {
          showError(message);
        }
      },
    });
  };

  const columns = useMemo(
    () => [
      {
        title: t('配置'),
        dataIndex: 'code',
        render: (text) => (
          <Tag color='light-blue' shape='circle'>
            {text}
          </Tag>
        ),
      },
      { title: t('供应商'), dataIndex: 'vendor_name' },
      { title: t('平台'), dataIndex: 'platform_type' },
      {
        title: t('折扣'),
        dataIndex: 'discount_label',
        render: (text, record) => text || record.discount_code || '-',
      },
      {
        title: t('操作'),
        render: (_, record) => (
          <Space>
            <Button
              size='small'
              type='tertiary'
              onClick={() => startEdit(record)}
            >
              {t('编辑')}
            </Button>
            <Button
              size='small'
              type='danger'
              onClick={() => deleteProfile(record)}
            >
              {t('删除')}
            </Button>
          </Space>
        ),
      },
    ],
    [t],
  );

  return (
    <Modal
      title={t('供应商配置')}
      visible={visible}
      onCancel={handleClose}
      footer={null}
      size='large'
    >
      <Form
        initValues={EMPTY_PROFILE}
        getFormApi={(api) => (formApiRef.current = api)}
        onSubmit={submit}
        layout='vertical'
      >
        <div className='grid grid-cols-1 md:grid-cols-3 gap-2'>
          <Form.Input
            field='code'
            label={t('配置代码')}
            placeholder='BP-AWS-D90'
            showClear
          />
          <Form.Input
            field='vendor_code'
            label={t('供应商代码')}
            placeholder='BP'
            rules={[{ required: true, message: t('供应商代码不能为空') }]}
            showClear
          />
          <Form.Input
            field='vendor_name'
            label={t('供应商名称')}
            placeholder='BytePlus'
            rules={[{ required: true, message: t('供应商名称不能为空') }]}
            showClear
          />
          <Form.Input
            field='platform_type'
            label={t('平台')}
            placeholder='AWS'
            rules={[{ required: true, message: t('平台不能为空') }]}
            showClear
          />
          <Form.Input
            field='discount_code'
            label={t('折扣代码')}
            placeholder='D90'
            showClear
          />
          <Form.Input
            field='discount_label'
            label={t('折扣显示')}
            placeholder={t('9折')}
            showClear
          />
          <Form.InputNumber
            field='discount_rate'
            label={t('折扣率')}
            min={0}
            step={0.01}
            precision={4}
            style={{ width: '100%' }}
          />
          <Form.Select
            field='status'
            label={t('状态')}
            optionList={[
              { label: t('启用'), value: 1 },
              { label: t('禁用'), value: 2 },
            ]}
          />
          <Form.Input field='notes' label={t('备注')} showClear />
        </div>
        <div className='flex justify-end gap-2 mt-2'>
          <Button type='tertiary' onClick={resetForm}>
            {t('新增')}
          </Button>
          <Button type='primary' htmlType='submit'>
            {editingProfile?.id ? t('更新') : t('保存')}
          </Button>
        </div>
      </Form>
      <Table
        className='mt-4'
        rowKey='id'
        loading={loading}
        columns={columns}
        dataSource={profiles}
        pagination={false}
      />
    </Modal>
  );
};

export default VendorProfileModal;
