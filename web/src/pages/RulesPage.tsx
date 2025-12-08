import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Badge } from '@/components/ui/badge';
import { Plus, Edit2, Trash2, Settings } from 'lucide-react';
import { useToast } from '@/hooks/use-toast';
import { useState } from 'react';
import type { AdvancedRule, UserRule } from '@/lib/types';
import AppLayout from '@/components/AppLayout';

export default function RulesPage() {
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<AdvancedRule | null>(null);
  const [deleteConfirmRule, setDeleteConfirmRule] = useState<string | null>(null);

  const { data: rulesData, isLoading } = useQuery({
    queryKey: ['rules'],
    queryFn: () => apiClient.listRules(),
  });

  const rules = rulesData?.rules || [];

  const createRuleMutation = useMutation({
    mutationFn: (rule: Omit<AdvancedRule, 'name'> & { name: string }) => apiClient.createRule(rule),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['rules'] });
      // Invalidate media queries to refresh deletion dates after rule changes
      queryClient.invalidateQueries({ queryKey: ['movies'] });
      queryClient.invalidateQueries({ queryKey: ['shows'] });
      queryClient.invalidateQueries({ queryKey: ['leaving-soon'] });
      queryClient.invalidateQueries({ queryKey: ['leaving-soon-all'] });
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
      setIsDialogOpen(false);
      setEditingRule(null);
      toast({
        title: 'Success',
        description: 'Rule created successfully',
      });
    },
    onError: (error: Error) => {
      toast({
        title: 'Error',
        description: error.message,
        variant: 'destructive',
      });
    },
  });

  const updateRuleMutation = useMutation({
    mutationFn: ({ name, rule }: { name: string; rule: Omit<AdvancedRule, 'name'> & { name: string } }) =>
      apiClient.updateRule(name, rule),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['rules'] });
      // Invalidate media queries to refresh deletion dates after rule changes
      queryClient.invalidateQueries({ queryKey: ['movies'] });
      queryClient.invalidateQueries({ queryKey: ['shows'] });
      queryClient.invalidateQueries({ queryKey: ['leaving-soon'] });
      queryClient.invalidateQueries({ queryKey: ['leaving-soon-all'] });
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
      setIsDialogOpen(false);
      setEditingRule(null);
      toast({
        title: 'Success',
        description: 'Rule updated successfully',
      });
    },
    onError: (error: Error) => {
      toast({
        title: 'Error',
        description: error.message,
        variant: 'destructive',
      });
    },
  });

  const deleteRuleMutation = useMutation({
    mutationFn: (name: string) => apiClient.deleteRule(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['rules'] });
      // Invalidate media queries to refresh deletion dates after rule changes
      queryClient.invalidateQueries({ queryKey: ['movies'] });
      queryClient.invalidateQueries({ queryKey: ['shows'] });
      queryClient.invalidateQueries({ queryKey: ['leaving-soon'] });
      queryClient.invalidateQueries({ queryKey: ['leaving-soon-all'] });
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
      setDeleteConfirmRule(null);
      toast({
        title: 'Success',
        description: 'Rule deleted successfully',
      });
    },
    onError: (error: Error) => {
      toast({
        title: 'Error',
        description: error.message,
        variant: 'destructive',
      });
    },
  });

  const toggleRuleMutation = useMutation({
    mutationFn: ({ name, enabled }: { name: string; enabled: boolean }) =>
      apiClient.toggleRule(name, enabled),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['rules'] });
      // Invalidate media queries to refresh deletion dates after rule changes
      queryClient.invalidateQueries({ queryKey: ['movies'] });
      queryClient.invalidateQueries({ queryKey: ['shows'] });
      queryClient.invalidateQueries({ queryKey: ['leaving-soon'] });
      queryClient.invalidateQueries({ queryKey: ['leaving-soon-all'] });
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
      toast({
        title: 'Success',
        description: 'Rule toggled successfully',
      });
    },
    onError: (error: Error) => {
      toast({
        title: 'Error',
        description: error.message,
        variant: 'destructive',
      });
    },
  });

  const handleAddRule = () => {
    setEditingRule(null);
    setIsDialogOpen(true);
  };

  const handleEditRule = (rule: AdvancedRule) => {
    setEditingRule(rule);
    setIsDialogOpen(true);
  };

  const handleDeleteRule = (name: string) => {
    setDeleteConfirmRule(name);
  };

  const confirmDelete = () => {
    if (deleteConfirmRule) {
      deleteRuleMutation.mutate(deleteConfirmRule);
    }
  };

  const handleToggleRule = (name: string, enabled: boolean) => {
    toggleRuleMutation.mutate({ name, enabled: !enabled });
  };

  if (isLoading) {
    return (
      <AppLayout>
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-8">
          <div className="text-center">Loading rules...</div>
        </div>
      </AppLayout>
    );
  }

  return (
    <AppLayout>
      <div className="container mx-auto px-4 py-6">
        <div className="flex justify-between items-center mb-6">
          <div className="flex items-center gap-4">
            <h1 className="text-3xl font-bold">Advanced Rules</h1>
          </div>
          <div className="flex gap-2">
            <Button onClick={handleAddRule} className="bg-blue-600 hover:bg-blue-700 text-white">
              <Plus className="h-4 w-4 mr-2" />
              Add Rule
            </Button>
          </div>
        </div>
        {/* Info Card */}
        <div className="bg-blue-900/20 border border-blue-900/50 rounded-md p-4">
          <h3 className="text-blue-400 font-semibold mb-3">About Advanced Rules</h3>
          <div className="text-sm text-gray-300 space-y-2">
            <p>• <strong className="text-white">Tag-based rules:</strong> Apply custom retention to media with specific tags</p>
            <p>• <strong className="text-white">Episode limit rules:</strong> Auto-delete old TV episodes beyond a max count</p>
            <p>• <strong className="text-white">User-based rules:</strong> Custom retention for content requested by specific users</p>
            <p>• Rules are evaluated in the order they appear in the config file</p>
          </div>
        </div>

        {/* Rules List */}
        {rules.length === 0 ? (
          <div className="bg-[#1a1a1a] border border-[#333] rounded-md py-12 text-center">
            <Settings className="h-12 w-12 mx-auto mb-4 text-gray-500" />
            <p className="text-lg font-medium text-white">No advanced rules configured</p>
            <p className="mt-2 text-gray-400">Click "Add Rule" to create your first rule</p>
          </div>
        ) : (
          <div className="grid gap-4">
            {rules.map((rule) => (
              <div key={rule.name} className="bg-[#1a1a1a] border border-[#333] rounded-md">
                <div className="p-4 border-b border-[#333]">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <h3 className="text-lg font-semibold text-white">{rule.name}</h3>
                      <Badge variant="outline" className={rule.enabled ? 'bg-green-900/20 text-green-400 border-green-900/50' : 'bg-[#262626] text-gray-400 border-[#444]'}>
                        {rule.enabled ? 'Enabled' : 'Disabled'}
                      </Badge>
                      <Badge variant="outline" className="bg-[#262626] text-gray-300 border-[#444] capitalize">{rule.type}</Badge>
                    </div>
                    <div className="flex gap-2">
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => handleToggleRule(rule.name, rule.enabled)}
                        className="text-gray-300 hover:text-white hover:bg-[#262626]"
                      >
                        {rule.enabled ? 'Disable' : 'Enable'}
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => handleEditRule(rule)}
                        className="text-gray-300 hover:text-white hover:bg-[#262626]"
                      >
                        <Edit2 className="h-4 w-4" />
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => handleDeleteRule(rule.name)}
                        className="text-red-400 hover:text-red-300 hover:bg-red-900/20"
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>
                </div>
                <div className="p-4">
                  <RuleDetails rule={rule} />
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Add/Edit Dialog */}
      {isDialogOpen && (
        <RuleDialog
          rule={editingRule}
          isOpen={isDialogOpen}
          onClose={() => {
            setIsDialogOpen(false);
            setEditingRule(null);
          }}
          onSave={(rule) => {
            if (editingRule) {
              updateRuleMutation.mutate({ name: editingRule.name, rule });
            } else {
              createRuleMutation.mutate(rule);
            }
          }}
        />
      )}

      {/* Delete Confirmation */}
      <Dialog open={!!deleteConfirmRule} onOpenChange={() => setDeleteConfirmRule(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Rule</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete the rule "{deleteConfirmRule}"? This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteConfirmRule(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={confirmDelete}
              disabled={deleteRuleMutation.isPending}
            >
              {deleteRuleMutation.isPending ? 'Deleting...' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </AppLayout>
  );
}

// Rule details component
function RuleDetails({ rule }: { rule: AdvancedRule }) {
  if (rule.type === 'tag') {
    return (
      <div className="space-y-1 text-sm text-gray-300">
        <p><strong className="text-white">Tag:</strong> {rule.tag}</p>
        <p><strong className="text-white">Retention:</strong> {rule.retention}</p>
      </div>
    );
  }

  if (rule.type === 'episode') {
    return (
      <div className="space-y-1 text-sm text-gray-300">
        <p><strong className="text-white">Max Episodes:</strong> {rule.max_episodes}</p>
        {rule.max_age && <p><strong className="text-white">Max Age:</strong> {rule.max_age}</p>}
        <p><strong className="text-white">Require Watched:</strong> {rule.require_watched ? 'Yes' : 'No'}</p>
      </div>
    );
  }

  if (rule.type === 'user') {
    return (
      <div className="space-y-2 text-sm text-gray-300">
        <p><strong className="text-white">Users:</strong></p>
        <ul className="list-disc list-inside pl-4 space-y-1">
          {rule.users?.map((user, idx) => (
            <li key={idx}>
              {user.username || user.email || `User ID: ${user.user_id}`} - {user.retention}
              {user.require_watched && ' (watched only)'}
            </li>
          ))}
        </ul>
      </div>
    );
  }

  return null;
}

// Rule dialog component
function RuleDialog({
  rule,
  isOpen,
  onClose,
  onSave,
}: {
  rule: AdvancedRule | null;
  isOpen: boolean;
  onClose: () => void;
  onSave: (rule: Omit<AdvancedRule, 'name'> & { name: string }) => void;
}) {
  const [formData, setFormData] = useState<Partial<AdvancedRule>>({
    name: rule?.name || '',
    type: rule?.type || 'tag',
    enabled: rule?.enabled ?? true,
    tag: rule?.tag || '',
    retention: rule?.retention || '90d',
    max_episodes: rule?.max_episodes || 10,
    max_age: rule?.max_age || '',
    require_watched: rule?.require_watched ?? false,
    users: rule?.users || [],
  });

  const [newUser, setNewUser] = useState<Partial<UserRule>>({
    username: '',
    retention: '90d',
    require_watched: false,
  });

  const handleSubmit = () => {
    if (!formData.name || !formData.type) {
      return;
    }

    const ruleData: Omit<AdvancedRule, 'name'> & { name: string } = {
      name: formData.name,
      type: formData.type as 'tag' | 'episode' | 'user',
      enabled: formData.enabled ?? true,
    };

    if (formData.type === 'tag') {
      ruleData.tag = formData.tag;
      ruleData.retention = formData.retention;
    } else if (formData.type === 'episode') {
      ruleData.max_episodes = formData.max_episodes;
      ruleData.max_age = formData.max_age;
      ruleData.require_watched = formData.require_watched;
    } else if (formData.type === 'user') {
      ruleData.users = formData.users;
    }

    onSave(ruleData);
  };

  const handleAddUser = () => {
    if (!newUser.username && !newUser.email && !newUser.user_id) {
      return;
    }

    setFormData({
      ...formData,
      users: [...(formData.users || []), newUser as UserRule],
    });

    setNewUser({
      username: '',
      retention: '90d',
      require_watched: false,
    });
  };

  const handleRemoveUser = (index: number) => {
    setFormData({
      ...formData,
      users: formData.users?.filter((_, i) => i !== index),
    });
  };

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{rule ? 'Edit Rule' : 'Add New Rule'}</DialogTitle>
          <DialogDescription>
            Configure an advanced retention rule for your media library
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {/* Name */}
          <div>
            <label className="text-sm font-medium">Rule Name</label>
            <Input
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              placeholder="my-rule"
              disabled={!!rule}
            />
            <p className="text-xs text-gray-500 mt-1">Unique identifier for this rule</p>
          </div>

          {/* Type */}
          <div>
            <label className="text-sm font-medium">Rule Type</label>
            <select
              value={formData.type}
              onChange={(e) => setFormData({ ...formData, type: e.target.value as any })}
              className="w-full border rounded-md px-3 py-2"
              disabled={!!rule}
            >
              <option value="tag">Tag-based</option>
              <option value="episode">Episode Limit</option>
              <option value="user">User-based</option>
            </select>
          </div>

          {/* Enabled */}
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={formData.enabled}
              onChange={(e) => setFormData({ ...formData, enabled: e.target.checked })}
              className="h-4 w-4"
            />
            <label className="text-sm font-medium">Enabled</label>
          </div>

          {/* Type-specific fields */}
          {formData.type === 'tag' && (
            <>
              <div>
                <label className="text-sm font-medium">Tag</label>
                <Input
                  value={formData.tag}
                  onChange={(e) => setFormData({ ...formData, tag: e.target.value })}
                  placeholder="4k"
                />
              </div>
              <div>
                <label className="text-sm font-medium">Retention Period</label>
                <Input
                  value={formData.retention}
                  onChange={(e) => setFormData({ ...formData, retention: e.target.value })}
                  placeholder="90d"
                />
                <p className="text-xs text-gray-500 mt-1">e.g., "30d", "90d", "180d"</p>
              </div>
            </>
          )}

          {formData.type === 'episode' && (
            <>
              <div>
                <label className="text-sm font-medium">Max Episodes</label>
                <Input
                  type="number"
                  value={formData.max_episodes}
                  onChange={(e) => setFormData({ ...formData, max_episodes: parseInt(e.target.value) })}
                  min="1"
                />
                <p className="text-xs text-gray-500 mt-1">Keep only this many recent episodes</p>
              </div>
              <div>
                <label className="text-sm font-medium">Max Age (optional)</label>
                <Input
                  value={formData.max_age}
                  onChange={(e) => setFormData({ ...formData, max_age: e.target.value })}
                  placeholder="30d"
                />
                <p className="text-xs text-gray-500 mt-1">Delete episodes older than this age</p>
              </div>
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={formData.require_watched}
                  onChange={(e) => setFormData({ ...formData, require_watched: e.target.checked })}
                  className="h-4 w-4"
                />
                <label className="text-sm font-medium">Require Watched</label>
                <p className="text-xs text-gray-500">(Only delete watched episodes)</p>
              </div>
            </>
          )}

          {formData.type === 'user' && (
            <>
              <div className="border rounded-md p-4 space-y-3">
                <p className="text-sm font-medium">Users</p>
                {formData.users && formData.users.length > 0 ? (
                  <ul className="space-y-2">
                    {formData.users.map((user, idx) => (
                      <li key={idx} className="flex items-center justify-between bg-gray-50 p-2 rounded">
                        <span className="text-sm">
                          {user.username || user.email || `User ID: ${user.user_id}`} - {user.retention}
                          {user.require_watched && ' (watched only)'}
                        </span>
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => handleRemoveUser(idx)}
                        >
                          <Trash2 className="h-3 w-3 text-red-500" />
                        </Button>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <p className="text-sm text-gray-500">No users added yet</p>
                )}
              </div>

              <div className="border rounded-md p-4 space-y-3">
                <p className="text-sm font-medium">Add User</p>
                <div>
                  <label className="text-xs text-gray-600">Username</label>
                  <Input
                    value={newUser.username}
                    onChange={(e) => setNewUser({ ...newUser, username: e.target.value })}
                    placeholder="john_doe"
                  />
                </div>
                <div>
                  <label className="text-xs text-gray-600">Email (optional)</label>
                  <Input
                    value={newUser.email}
                    onChange={(e) => setNewUser({ ...newUser, email: e.target.value })}
                    placeholder="john@example.com"
                  />
                </div>
                <div>
                  <label className="text-xs text-gray-600">User ID (optional)</label>
                  <Input
                    type="number"
                    value={newUser.user_id || ''}
                    onChange={(e) => setNewUser({ ...newUser, user_id: parseInt(e.target.value) })}
                    placeholder="123"
                  />
                </div>
                <div>
                  <label className="text-xs text-gray-600">Retention</label>
                  <Input
                    value={newUser.retention}
                    onChange={(e) => setNewUser({ ...newUser, retention: e.target.value })}
                    placeholder="90d"
                  />
                </div>
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={newUser.require_watched}
                    onChange={(e) => setNewUser({ ...newUser, require_watched: e.target.checked })}
                    className="h-4 w-4"
                  />
                  <label className="text-xs text-gray-600">Require Watched</label>
                </div>
                <Button size="sm" onClick={handleAddUser}>
                  Add User
                </Button>
              </div>
            </>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={handleSubmit}>
            {rule ? 'Update Rule' : 'Create Rule'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
