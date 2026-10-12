package session

// OwnerSession identifies the connection that receives this inventory command.
func (c *giveItemCommand) OwnerSession() Session { return c.session }

// OwnerSession identifies the connection that owns this queued action.
func (c *movementCommand) OwnerSession() Session { return c.session }

// OwnerSession identifies the connection that owns this queued action.
func (c *teleportAckCommand) OwnerSession() Session { return c.session }

// OwnerSession identifies the connection that owns this queued action.
func (c *clientLoadedCommand) OwnerSession() Session { return c.session }

// preserveDisconnectOrder retains accepted movement until the queued disconnect snapshot.
func (c *movementCommand) preserveDisconnectOrder() {}

// preserveDisconnectOrder retains teleport acknowledgements preceding accepted movement.
func (c *teleportAckCommand) preserveDisconnectOrder() {}

// preserveDisconnectOrder retains terrain-load acknowledgements preceding accepted movement.
func (c *clientLoadedCommand) preserveDisconnectOrder() {}

// OwnerSession identifies the connection that owns this queued action.
func (c *digActionCommand) OwnerSession() Session { return c.session }

// OwnerSession identifies the connection that owns this queued action.
func (c *heldSlotCommand) OwnerSession() Session { return c.session }

// OwnerSession identifies the connection that owns this queued action.
func (c *creativeSlotCommand) OwnerSession() Session { return c.session }

// OwnerSession identifies the connection that owns this queued action.
func (c *closeWindowCommand) OwnerSession() Session { return c.session }

// OwnerSession identifies the connection that owns this queued action.
func (c *openBlockWindowCommand) OwnerSession() Session { return c.session }

// OwnerSession identifies the connection that owns this queued action.
func (c *recipePlacementCommand) OwnerSession() Session { return c.session }

// OwnerSession identifies the connection that owns this queued action.
func (c *ignitionCommand) OwnerSession() Session { return c.session }

// OwnerSession identifies the connection that owns this queued action.
func (c *bedUseCommand) OwnerSession() Session { return c.session }

// OwnerSession identifies the connection that owns this queued action.
func (c *wakeBedCommand) OwnerSession() Session { return c.session }

// OwnerSession identifies the connection that owns this queued action.
func (c *respawnPlayerCommand) OwnerSession() Session { return c.session }

// OwnerSession identifies the connection that owns this queued action.
func (c *useItemCommand) OwnerSession() Session { return c.session }

// OwnerSession identifies the connection that owns this queued action.
func (c *windowClickCommand) OwnerSession() Session { return c.session }

// OwnerSession identifies the connection that owns this queued action.
func (c *placeBlockCommand) OwnerSession() Session { return c.session }
