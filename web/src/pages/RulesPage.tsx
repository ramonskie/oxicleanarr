import { useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { useAuthStore } from '@/store/auth';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Badge } from '@/components/ui/badge';
import { LogOut, Plus, Edit2, Trash2, Settings } from 'lucide-react';
import { useToast } from '@/hooks/use-toast';
import { useState } from 'react';
import type { AdvancedRule, UserRule } from '@/lib/types';

export default function RulesPage() {
  const navigate = useNavigate();
  const logout = useAuthStore((state) => state.logout);
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

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

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
      <div className="min-h-screen bg-gray-50">
        <header className="bg-white shadow">
          <div className="mx-auto px-4 sm:px-6 lg:px-8 py-6 flex justify-between items-center">
            <h1 className="text-3xl font-bold text-gray-900">Advanced Rules</h1>
          </div>
        </header>
        <main className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-8">
          <div className="text-center">Loading rules...</div>
        </main>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white shadow">
        <div className="mx-auto px-4 sm:px-6 lg:px-8 py-6 flex justify-between items-center">
          <div className="flex items-center gap-4">
            <Button variant="ghost" onClick={() => navigate('/configuration')}>
              ← Back to Configuration
            </Button>
            <h1 className="text-3xl font-bold text-gray-900">Advanced Rules</h1>
          </div>
          <div className="flex gap-2">
            <Button onClick={handleAddRule}>
              <Plus className="h-4 w-4 mr-2" />
              Add Rule
            </Button>
            <Button onClick={handleLogout} variant="outline">
              <LogOut className="h-4 w-4 mr-2" />
              Logout
            </Button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-8">
        <div className="space-y-6">
          {/* Info Card */}
          <Card className="bg-blue-50 border-blue-200">
            <CardHeader>
              <CardTitle className="text-blue-900">About Advanced Rules</CardTitle>
            </CardHeader>
            <CardContent className="text-sm text-blue-800 space-y-2">
              <p>• <strong>Tag-based rules:</strong> Apply custom retention to media with specific tags</p>
              <p>• <strong>Episode limit rules:</strong> Auto-delete old TV episodes beyond a max count</p>
              <p>• <strong>User-based rules:</strong> Custom retention for content requested by specific users</p>
              <p>• Rules are evaluated in the order they appear in the config file</p>
            </CardContent>
          </Card>

          {/* Rules List */}
          {rules.length === 0 ? (
            <Card>
              <CardContent className="py-12 text-center text-gray-500">
                <Settings className="h-12 w-12 mx-auto mb-4 text-gray-400" />
                <p className="text-lg font-medium">No advanced rules configured</p>
                <p className="mt-2">Click "Add Rule" to create your first rule</p>
              </CardContent>
            </Card>
          ) : (
            <div className="grid gap-4">
              {rules.map((rule) => (
                <Card key={rule.name}>
                  <CardHeader>
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <CardTitle>{rule.name}</CardTitle>
                        <Badge variant={rule.enabled ? 'default' : 'secondary'}>
                          {rule.enabled ? 'Enabled' : 'Disabled'}
                        </Badge>
                        <Badge variant="outline">{rule.type}</Badge>
                      </div>
                      <div className="flex gap-2">
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => handleToggleRule(rule.name, rule.enabled)}
                        >
                          {rule.enabled ? 'Disable' : 'Enable'}
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => handleEditRule(rule)}
                        >
                          <Edit2 className="h-4 w-4" />
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => handleDeleteRule(rule.name)}
                        >
                          <Trash2 className="h-4 w-4 text-red-500" />
                        </Button>
                      </div>
                    </div>
                  </CardHeader>
                  <CardContent>
                    <RuleDetails rule={rule} />
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </div>
      </main>

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
    </div>
  );
}

// Rule details component
function RuleDetails({ rule }: { rule: AdvancedRule }) {
  if (rule.type === 'tag') {
    return (
      <div className="space-y-1 text-sm">
        <p><strong>Tag:</strong> {rule.tag}</p>
        <p><strong>Retention:</strong> {rule.retention}</p>
      </div>
    );
  }

  if (rule.type === 'episode') {
    return (
      <div className="space-y-1 text-sm">
        <p><strong>Max Episodes:</strong> {rule.max_episodes}</p>
        {rule.max_age && <p><strong>Max Age:</strong> {rule.max_age}</p>}
        <p><strong>Require Watched:</strong> {rule.require_watched ? 'Yes' : 'No'}</p>
      </div>
    );
  }

  if (rule.type === 'user') {
    return (
      <div className="space-y-2 text-sm">
        <p><strong>Users:</strong></p>
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
