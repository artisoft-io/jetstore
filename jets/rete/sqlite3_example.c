
// OPEN
// ===========================
bool QgsOSMDatabase::open()
{
  // load spatialite extension
  spatialite_init( 0 );

  // open database
  int res = sqlite3_open_v2( mDbFileName.toUtf8().data(), &mDatabase, SQLITE_OPEN_READWRITE, 0 );
  if ( res != SQLITE_OK )
  {
    mError = QString( "Failed to open database [%1]: %2" ).arg( res ).arg( mDbFileName );
    close();
    return false;
  }

  if ( !prepareStatements() )
  {
    close();
    return false;
  }

  return true;
}


// DELETE
void QgsOSMDatabase::deleteStatement( sqlite3_stmt*& stmt )
{
  if ( stmt )
  {
    sqlite3_finalize( stmt );
    stmt = 0;
  }
}

// PREPARE STATEMENT
// ===========================
bool QgsOSMDatabase::prepareStatements()
{
  const char* sql[] =
  {
    "SELECT lon,lat FROM nodes WHERE id=?",
    "SELECT k,v FROM nodes_tags WHERE id=?",
    "SELECT id FROM ways WHERE id=?",
    "SELECT node_id FROM ways_nodes WHERE way_id=? ORDER BY way_pos",
    "SELECT n.lon, n.lat FROM ways_nodes wn LEFT JOIN nodes n ON wn.node_id = n.id WHERE wn.way_id=? ORDER BY wn.way_pos",
    "SELECT k,v FROM ways_tags WHERE id=?"
  };
  sqlite3_stmt** sqlite[] =
  {
    &mStmtNode,
    &mStmtNodeTags,
    &mStmtWay,
    &mStmtWayNode,
    &mStmtWayNodePoints,
    &mStmtWayTags
  };
  int count = sizeof( sql ) / sizeof( const char* );
  Q_ASSERT( count == sizeof( sqlite ) / sizeof( sqlite3_stmt** ) );

  for ( int i = 0; i < count; ++i )
  {
    if ( sqlite3_prepare_v2( mDatabase, sql[i], -1, sqlite[i], 0 ) != SQLITE_OK )
    {
      const char* errMsg = sqlite3_errmsg( mDatabase ); // does not require free
      mError = QString( "Error preparing SQL command:\n%1\nSQL:\n%2" )
               .arg( QString::fromUtf8( errMsg ) ).arg( QString::fromUtf8( sql[i] ) );
      return false;
    }
  }

  return true;
}



// CLOSE
// ===========================
bool QgsOSMDatabase::close()
{
  deleteStatement( mStmtNode );
  deleteStatement( mStmtNodeTags );
  deleteStatement( mStmtWay );
  deleteStatement( mStmtWayNode );
  deleteStatement( mStmtWayNodePoints );
  deleteStatement( mStmtWayTags );

  Q_ASSERT( mStmtNode == 0 );

  // close database
  if ( sqlite3_close( mDatabase ) != SQLITE_OK )
  {
    //mError = ( char * ) "Closing SQLite3 database failed.";
    //return false;
  }
  mDatabase = 0;
  return true;
}


// RUN STATEMENT
// ===========================
int QgsOSMDatabase::runCountStatement( const char* sql ) const
{
  sqlite3_stmt* stmt;
  int res = sqlite3_prepare_v2( mDatabase, sql, -1, &stmt, 0 );
  if ( res != SQLITE_OK )
    return -1;

  res = sqlite3_step( stmt );
  if ( res != SQLITE_ROW )
    return -1;

  int count = sqlite3_column_int( stmt, 0 );
  sqlite3_finalize( stmt );
  return count;
}


int QgsOSMDatabase::countNodes() const
{
  return runCountStatement( "SELECT count(*) FROM nodes" );
}

int QgsOSMDatabase::countWays() const
{
  return runCountStatement( "SELECT count(*) FROM ways" );
}


QgsOSMNodeIterator QgsOSMDatabase::listNodes() const
{
  return QgsOSMNodeIterator( mDatabase );
}

QgsOSMWayIterator QgsOSMDatabase::listWays() const
{
  return QgsOSMWayIterator( mDatabase );
}

// RUN PREPARED STATEMENT
// ===========================
QgsOSMNode QgsOSMDatabase::node( QgsOSMId id ) const
{
  // bind the way identifier
  sqlite3_bind_int64( mStmtNode, 1, id );

  if ( sqlite3_step( mStmtNode ) != SQLITE_ROW )
  {
    //QgsDebugMsg( "Cannot get number of way members." );
    sqlite3_reset( mStmtNode );
    return QgsOSMNode();
  }

  double lon = sqlite3_column_double( mStmtNode, 0 );
  double lat = sqlite3_column_double( mStmtNode, 1 );

  QgsOSMNode node( id, QgsPoint( lon, lat ) );

  sqlite3_reset( mStmtNode );
  return node;
}

QgsOSMTags QgsOSMDatabase::tags( bool way, QgsOSMId id ) const
{
  QgsOSMTags t;

  sqlite3_stmt* stmtTags = way ? mStmtWayTags : mStmtNodeTags;

  sqlite3_bind_int64( stmtTags, 1, id );

  while ( sqlite3_step( stmtTags ) == SQLITE_ROW )
  {
    QString k = QString::fromUtf8(( const char* ) sqlite3_column_text( stmtTags, 0 ) );
    QString v = QString::fromUtf8(( const char* ) sqlite3_column_text( stmtTags, 1 ) );
    t.insert( k, v );
  }

  sqlite3_reset( stmtTags );
  return t;
}


QList<QgsOSMTagCountPair> QgsOSMDatabase::usedTags( bool ways ) const
{
  QList<QgsOSMTagCountPair> pairs;

  QString sql = QString( "SELECT k, count(k) FROM %1_tags GROUP BY k" ).arg( ways ? "ways" : "nodes" );

  sqlite3_stmt* stmt;
  if ( sqlite3_prepare_v2( mDatabase, sql.toUtf8().data(), -1, &stmt, 0 ) != SQLITE_OK )
    return pairs;

  while ( sqlite3_step( stmt ) == SQLITE_ROW )
  {
    QString k = QString::fromUtf8(( const char* ) sqlite3_column_text( stmt, 0 ) );
    int count = sqlite3_column_int( stmt, 1 );
    pairs.append( qMakePair( k, count ) );
  }

  sqlite3_finalize( stmt );
  return pairs;
}



QgsOSMWay QgsOSMDatabase::way( QgsOSMId id ) const
{
  // TODO: first check that way exists!
  // mStmtWay

  // bind the way identifier
  sqlite3_bind_int64( mStmtWayNode, 1, id );

  QList<QgsOSMId> nodes;

  while ( sqlite3_step( mStmtWayNode ) == SQLITE_ROW )
  {
    QgsOSMId nodeId = sqlite3_column_int64( mStmtWayNode, 0 );
    nodes.append( nodeId );
  }

  sqlite3_reset( mStmtWayNode );

  if ( nodes.isEmpty() )
    return QgsOSMWay();

  return QgsOSMWay( id, nodes );
}

/*
OSMRelation OSMDatabase::relation( OSMId id ) const
{
  // todo
  Q_UNUSED(id);
  return OSMRelation();
}*/

QgsPolyline QgsOSMDatabase::wayPoints( QgsOSMId id ) const
{
  QgsPolyline points;

  // bind the way identifier
  sqlite3_bind_int64( mStmtWayNodePoints, 1, id );

  while ( sqlite3_step( mStmtWayNodePoints ) == SQLITE_ROW )
  {
    if ( sqlite3_column_type( mStmtWayNodePoints, 0 ) == SQLITE_NULL )
      return QgsPolyline(); // missing some nodes
    double lon = sqlite3_column_double( mStmtWayNodePoints, 0 );
    double lat = sqlite3_column_double( mStmtWayNodePoints, 1 );
    points.append( QgsPoint( lon, lat ) );
  }

  sqlite3_reset( mStmtWayNodePoints );
  return points;
}
